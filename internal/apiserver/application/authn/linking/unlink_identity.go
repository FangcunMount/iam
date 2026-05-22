package linking

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Unlink 解绑登录身份；最后一个活跃登录身份不允许解绑。
func (l *linker) Unlink(ctx context.Context, cmd UnlinkCommand) error {
	if err := validateUnlinkCommand(cmd); err != nil {
		return err
	}

	// 加载属于当前用户的登录身份。
	identity, err := l.loadOwnedLoginIdentity(ctx, cmd)
	if err != nil {
		return err
	}

	// 检查是否为最后一个活跃登录身份。
	if err := l.ensureNotLastActiveIdentity(ctx, cmd.UserID, identity); err != nil {
		return err
	}

	// 检查是否需要重新认证。
	if err := l.ensureUnlinkReauthentication(identity, cmd); err != nil {
		return err
	}

	// 更新登录身份状态为已删除。
	return l.repo().UpdateStatus(ctx, identity.ID, loginidentity.StatusDeleted)
}

// validateUnlinkCommand 验证解绑命令。
func validateUnlinkCommand(cmd UnlinkCommand) error {
	// 检查用户 ID 是否有效。
	if err := requireUserID(cmd.UserID); err != nil {
		return err
	}
	if cmd.LoginIdentityID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "login_identity_id is required")
	}
	return nil
}

// loadOwnedLoginIdentity 加载属于当前用户的登录身份。
func (l *linker) loadOwnedLoginIdentity(ctx context.Context, cmd UnlinkCommand) (*loginidentity.LoginIdentity, error) {
	// 查询登录身份。
	identity, err := l.repo().GetByID(ctx, cmd.LoginIdentityID)
	if err != nil {
		return nil, err
	}
	// 检查登录身份是否属于当前用户。
	if identity == nil || identity.UserID != cmd.UserID {
		return nil, perrors.WithCode(code.ErrLoginIdentityNotFound, "login identity not found")
	}
	// 返回登录身份。
	return identity, nil
}

// ensureNotLastActiveIdentity 检查是否为最后一个活跃登录身份。
func (l *linker) ensureNotLastActiveIdentity(ctx context.Context, userID meta.ID, identity *loginidentity.LoginIdentity) error {
	// 检查登录身份是否活跃。
	if identity.Status != loginidentity.StatusActive {
		return nil
	}

	// 查询用户登录身份。
	identities, err := l.repo().ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	// 检查是否为最后一个活跃登录身份。
	if countActive(identities) <= 1 {
		return perrors.WithCode(code.ErrInvalidArgument, "cannot unlink the last active login identity")
	}
	return nil
}

// ensureUnlinkReauthentication 检查是否需要重新认证。
func (l *linker) ensureUnlinkReauthentication(identity *loginidentity.LoginIdentity, cmd UnlinkCommand) error {
	// 检查是否需要重新认证。
	if l.requiresRecentAuthentication(identity, cmd) && !l.hasRecentAuthentication(cmd.AuthenticatedAt) {
		return perrors.WithCode(code.ErrReauthenticationRequired, "recent authentication required")
	}
	return nil
}

// requiresRecentAuthentication 检查是否需要重新认证。
func (l *linker) requiresRecentAuthentication(identity *loginidentity.LoginIdentity, cmd UnlinkCommand) bool {
	if identity == nil {
		return false
	}
	if !cmd.CurrentLoginIdentityID.IsZero() && identity.ID == cmd.CurrentLoginIdentityID {
		return true
	}
	switch identity.Provider {
	case loginidentity.ProviderUsername, loginidentity.ProviderPhone:
		return true
	default:
		return false
	}
}

// hasRecentAuthentication 检查是否在最近认证窗口内。
func (l *linker) hasRecentAuthentication(authenticatedAt *time.Time) bool {
	if authenticatedAt == nil || authenticatedAt.IsZero() {
		return false
	}
	now := l.now().UTC()
	authAt := authenticatedAt.UTC()
	if authAt.After(now.Add(time.Minute)) {
		return false
	}
	return now.Sub(authAt) <= l.recentAuthWindow()
}

// countActive 计算活跃登录身份数量。
func countActive(identities []*loginidentity.LoginIdentity) int {
	count := 0
	for _, identity := range identities {
		if identity != nil && identity.Status == loginidentity.StatusActive {
			count++
		}
	}
	return count
}
