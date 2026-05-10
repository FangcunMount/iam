package linking

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// UnlinkCommand 是解绑登录身份命令。
type UnlinkCommand struct {
	UserID                 meta.ID
	LoginIdentityID        meta.ID
	CurrentLoginIdentityID meta.ID
	AuthenticatedAt        *time.Time
}

// Unlink 解绑登录身份；最后一个 active 登录身份不允许解绑。
func (s *service) Unlink(ctx context.Context, cmd UnlinkCommand) error {
	if err := requireUserID(cmd.UserID); err != nil {
		return err
	}
	if cmd.LoginIdentityID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "login_identity_id is required")
	}
	identity, err := s.repo().GetByID(ctx, cmd.LoginIdentityID)
	if err != nil {
		return err
	}
	if identity == nil || identity.UserID != cmd.UserID {
		return perrors.WithCode(code.ErrLoginIdentityNotFound, "login identity not found")
	}
	if identity.Status == loginidentity.StatusActive {
		identities, err := s.repo().ListByUserID(ctx, cmd.UserID)
		if err != nil {
			return err
		}
		if countActive(identities) <= 1 {
			return perrors.WithCode(code.ErrInvalidArgument, "cannot unlink the last active login identity")
		}
	}
	if s.requiresRecentAuthentication(identity, cmd) && !s.hasRecentAuthentication(cmd.AuthenticatedAt) {
		return perrors.WithCode(code.ErrReauthenticationRequired, "recent authentication required")
	}
	return s.repo().UpdateStatus(ctx, identity.ID, loginidentity.StatusDeleted)
}

func (s *service) requiresRecentAuthentication(identity *loginidentity.LoginIdentity, cmd UnlinkCommand) bool {
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

func (s *service) hasRecentAuthentication(authenticatedAt *time.Time) bool {
	if authenticatedAt == nil || authenticatedAt.IsZero() {
		return false
	}
	now := s.now().UTC()
	authAt := authenticatedAt.UTC()
	if authAt.After(now.Add(time.Minute)) {
		return false
	}
	return now.Sub(authAt) <= s.recentAuthWindow()
}

func countActive(identities []*loginidentity.LoginIdentity) int {
	count := 0
	for _, identity := range identities {
		if identity != nil && identity.Status == loginidentity.StatusActive {
			count++
		}
	}
	return count
}
