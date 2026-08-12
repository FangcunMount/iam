package session

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Store 负责持久化认证会话与批量索引。
type Store interface {
	// —— 保存会话 —— //
	Save(ctx context.Context, session *Session) error

	// —— 查询会话 —— //
	Get(ctx context.Context, sessionID string) (*Session, error)

	// —— 撤销会话 —— //
	Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error

	// —— 延长会话过期时间 —— //
	Extend(ctx context.Context, sessionID string, expiresAt time.Time) error
	RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error
	RevokeByLoginIdentity(ctx context.Context, loginIdentityID meta.ID, reason string, revokedBy string) error
}
