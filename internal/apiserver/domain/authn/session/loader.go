package session

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// loader 用于加载会话。
type loader struct {
	store    Store
	lifetime LifetimePolicy
}

// newLoader 创建 loader。
func newLoader(store Store, lifetime LifetimePolicy) *loader {
	return &loader{store: store, lifetime: lifetime}
}

// NewLoader 创建会话加载器。
func NewLoader(store Store, lifetime LifetimePolicy) Loader {
	return newLoader(store, lifetime)
}

// Get 获取会话。
func (l *loader) Get(ctx context.Context, sessionID string) (*Session, error) {
	return l.store.Get(ctx, sessionID)
}

// GetActive 获取活跃会话。
func (l *loader) GetActive(ctx context.Context, sessionID string) (*Session, error) {
	session, err := l.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || !session.IsActive() {
		return nil, perrors.WithCode(code.ErrSessionInactive, "session has been revoked or expired")
	}
	if err := l.lifetime.EnsureActiveWithinLifetime(time.Now().UTC(), session); err != nil {
		return nil, err
	}
	return session, nil
}
