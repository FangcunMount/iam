package session

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type sessionApplicationService struct {
	manager sessiondomain.Manager
}

// NewSessionApplicationService 创建会话应用服务。
func NewSessionApplicationService(manager sessiondomain.Manager) SessionApplicationService {
	return &sessionApplicationService{manager: manager}
}

func (s *sessionApplicationService) RevokeSession(ctx context.Context, sessionID string, reason string, revokedBy string) error {
	l := logger.L(ctx)
	if sessionID == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "session_id is required")
	}
	l.Debugw("撤销单个会话",
		"action", logger.ActionRevoke,
		"resource", "session",
		"session_id", sessionID,
	)
	return s.manager.Revoke(ctx, sessionID, normalizeReason(reason, "admin_revoked_session"), revokedBy)
}

func (s *sessionApplicationService) RevokeAllSessionsByLoginIdentity(ctx context.Context, loginIdentityID string, reason string, revokedBy string) error {
	l := logger.L(ctx)
	id, err := parseMetaID(loginIdentityID)
	if err != nil {
		return err
	}
	l.Debugw("按登录身份撤销全部会话",
		"action", logger.ActionRevoke,
		"resource", "session",
		"login_identity_id", loginIdentityID,
	)
	return s.manager.RevokeByLoginIdentity(ctx, id, normalizeReason(reason, "admin_revoked_login_identity_sessions"), revokedBy)
}

func (s *sessionApplicationService) RevokeAllSessionsByUser(ctx context.Context, userID string, reason string, revokedBy string) error {
	l := logger.L(ctx)
	id, err := parseMetaID(userID)
	if err != nil {
		return err
	}
	l.Debugw("按用户撤销全部会话",
		"action", logger.ActionRevoke,
		"resource", "session",
		"user_id", userID,
	)
	return s.manager.RevokeByUser(ctx, id, normalizeReason(reason, "admin_revoked_user_sessions"), revokedBy)
}

func parseMetaID(raw string) (meta.ID, error) {
	var id uint64
	if _, err := fmt.Sscanf(raw, "%d", &id); err != nil {
		return meta.FromUint64(0), perrors.WithCode(code.ErrInvalidArgument, "invalid id: %s", raw)
	}
	return meta.FromUint64(id), nil
}

func normalizeReason(reason string, fallback string) string {
	if reason == "" {
		return fallback
	}
	return reason
}
