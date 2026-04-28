package session

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Store 负责持久化认证会话与批量索引。
type Store interface {
	Save(ctx context.Context, session *Session) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error
	Extend(ctx context.Context, sessionID string, expiresAt time.Time) error
	RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error
	RevokeByAccount(ctx context.Context, accountID meta.ID, reason string, revokedBy string) error
}
