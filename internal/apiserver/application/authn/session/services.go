package session

import "context"

// SessionApplicationService 提供管理员会话控制动作。
type SessionApplicationService interface {
	// RevokeSession 撤销单个会话。
	RevokeSession(ctx context.Context, sessionID string, reason string, revokedBy string) error
	// RevokeAllSessionsByLoginIdentity 撤销指定登录身份下的全部活跃会话。
	RevokeAllSessionsByLoginIdentity(ctx context.Context, loginIdentityID string, reason string, revokedBy string) error
	// RevokeAllSessionsByUser 撤销指定用户下的全部活跃会话。
	RevokeAllSessionsByUser(ctx context.Context, userID string, reason string, revokedBy string) error
}
