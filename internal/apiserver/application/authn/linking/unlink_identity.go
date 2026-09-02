package linking

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
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

	// 检查是否需要重新认证。
	if err := l.ensureUnlinkReauthentication(identity, cmd); err != nil {
		return err
	}

	unlinker := l.identityUnlinker()
	if unlinker == nil {
		return perrors.WithCode(code.ErrInternalServerError, "atomic login identity unlinker is required")
	}
	outcome, err := unlinker.UnlinkOwnedUnlessLastActive(ctx, cmd.UserID, identity.ID)
	if err != nil {
		return err
	}
	switch outcome {
	case loginidentity.UnlinkOutcomeUnlinked:
		return nil
	case loginidentity.UnlinkOutcomeNotFound:
		return perrors.WithCode(code.ErrLoginIdentityNotFound, "login identity not found")
	case loginidentity.UnlinkOutcomeLastActive:
		return perrors.WithCode(code.ErrInvalidArgument, "cannot unlink the last active login identity")
	default:
		return perrors.WithCode(code.ErrInternalServerError, "unexpected login identity unlink outcome")
	}
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

// ensureUnlinkReauthentication 检查是否需要重新认证。
func (l *linker) ensureUnlinkReauthentication(identity *loginidentity.LoginIdentity, cmd UnlinkCommand) error {
	policy := loginidentity.UnlinkPolicy{
		RecentAuthWindow: l.recentAuthWindow(),
		FutureClockSkew:  loginidentity.DefaultFutureClockSkew,
	}
	switch policy.AssessRecentAuthentication(loginidentity.UnlinkReauthRequest{
		Identity:               identity,
		CurrentLoginIdentityID: cmd.CurrentLoginIdentityID,
		AuthenticatedAt:        cmd.AuthenticatedAt,
		Now:                    l.now(),
	}) {
	case loginidentity.UnlinkReauthOK:
		return nil
	case loginidentity.UnlinkReauthRequired, loginidentity.UnlinkReauthInvalidTimestamp:
		return perrors.WithCode(code.ErrReauthenticationRequired, "recent authentication required")
	default:
		return perrors.WithCode(code.ErrInternalServerError, "unexpected unlink reauthentication decision")
	}
}
