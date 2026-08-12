package session

import (
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// LifetimePolicy 用于管理会话的生命周期。
type LifetimePolicy struct {
	refreshTTL    time.Duration
	sessionMaxTTL time.Duration
}

// NewLifetimePolicy 创建会话生命周期策略。
//
// refreshTTL 控制正常刷新令牌窗口。
// sessionMaxTTL 是登录会话的绝对生命周期；非正数表示没有超过当前会话过期时间的绝对上限。
func NewLifetimePolicy(refreshTTL, sessionMaxTTL time.Duration) LifetimePolicy {
	return LifetimePolicy{
		refreshTTL:    refreshTTL,
		sessionMaxTTL: sessionMaxTTL,
	}
}

// InitialExpiresAt 返回创建会话时的过期时间。
func (p LifetimePolicy) InitialExpiresAt(now time.Time) (time.Time, error) {
	if p.refreshTTL <= 0 {
		return time.Time{}, perrors.WithCode(code.ErrInvalidArgument, "refresh token ttl must be positive")
	}
	expiresAt := now.Add(p.refreshTTL)
	if p.sessionMaxTTL > 0 {
		expiresAt = minTime(expiresAt, now.Add(p.sessionMaxTTL))
	}
	if !expiresAt.After(now) {
		return time.Time{}, perrors.WithCode(code.ErrInvalidArgument, "session ttl must be positive")
	}
	return expiresAt, nil
}

// RefreshTokenExpiresAt 返回新旋转的刷新令牌的过期时间。
func (p LifetimePolicy) RefreshTokenExpiresAt(now time.Time, sess *Session) (time.Time, error) {
	if p.refreshTTL <= 0 {
		return time.Time{}, perrors.WithCode(code.ErrInvalidArgument, "refresh token ttl must be positive")
	}
	expiresAt := now.Add(p.refreshTTL)
	if limit, ok := p.ExpiryLimit(sess); ok {
		expiresAt = minTime(expiresAt, limit)
	}
	if !expiresAt.After(now) {
		return time.Time{}, perrors.WithCode(code.ErrSessionInactive, "session maximum lifetime exceeded")
	}
	return expiresAt, nil
}

// ExtensionExpiresAt 返回请求的会话延期的过期时间。
func (p LifetimePolicy) ExtensionExpiresAt(now time.Time, sess *Session, requestedExpiresAt time.Time) (time.Time, error) {
	if requestedExpiresAt.IsZero() {
		return time.Time{}, perrors.WithCode(code.ErrInvalidArgument, "session extension expires_at is required")
	}
	expiresAt := requestedExpiresAt
	if limit, ok := p.ExpiryLimit(sess); ok {
		expiresAt = minTime(expiresAt, limit)
	}
	if !expiresAt.After(now) {
		return time.Time{}, perrors.WithCode(code.ErrSessionInactive, "session maximum lifetime exceeded")
	}
	return expiresAt, nil
}

// EnsureActiveWithinLifetime 返回 ErrSessionInactive 当会话已超过其绝对生命周期边界时。
// crossed its absolute lifetime boundary.
func (p LifetimePolicy) EnsureActiveWithinLifetime(now time.Time, sess *Session) error {
	limit, ok := p.ExpiryLimit(sess)
	if !ok || now.Before(limit) {
		return nil
	}
	return perrors.WithCode(code.ErrSessionInactive, "session maximum lifetime exceeded")
}

// ExpiryLimit 返回会话的最强已知过期边界。
func (p LifetimePolicy) ExpiryLimit(sess *Session) (time.Time, bool) {
	if sess == nil {
		return time.Time{}, false
	}
	limit := sess.ExpiresAt
	if p.sessionMaxTTL > 0 && !sess.CreatedAt.IsZero() {
		maxExpiresAt := sess.CreatedAt.Add(p.sessionMaxTTL)
		if limit.IsZero() || maxExpiresAt.Before(limit) {
			limit = maxExpiresAt
		}
	}
	if limit.IsZero() {
		return time.Time{}, false
	}
	return limit, true
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
