package session

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// extender 用于延长会话过期时间。
type extender struct {
	store    Store
	lifetime LifetimePolicy
}

// newExtender 创建 extender。
func newExtender(store Store, lifetime LifetimePolicy) *extender {
	return &extender{store: store, lifetime: lifetime}
}

// NewExtender 创建会话延期器。
func NewExtender(store Store, lifetime LifetimePolicy) Extender {
	return newExtender(store, lifetime)
}

// Extend 延长会话过期时间。
func (e *extender) Extend(ctx context.Context, sessionID string, expiresAt time.Time) error {
	return e.store.Extend(ctx, sessionID, expiresAt)
}

// ExtendToRefreshExpiry 延长会话到 refresh token 过期时间。
func (e *extender) ExtendToRefreshExpiry(ctx context.Context, session *Session, refreshExpiresAt time.Time) error {
	if session == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "session is required")
	}
	expiresAt, err := e.lifetime.ExtensionExpiresAt(time.Now().UTC(), session, refreshExpiresAt)
	if err != nil {
		return err
	}
	return e.Extend(ctx, session.SessionID, expiresAt)
}
