package session

import "context"

// SessionApplicationService 提供管理员会话控制动作。
type SessionApplicationService interface {
	RevokeSession(ctx context.Context, sessionID string, reason string, revokedBy string) error
	RevokeAllSessionsByLoginIdentity(ctx context.Context, loginIdentityID string, reason string, revokedBy string) error
	RevokeAllSessionsByUser(ctx context.Context, userID string, reason string, revokedBy string) error
}
