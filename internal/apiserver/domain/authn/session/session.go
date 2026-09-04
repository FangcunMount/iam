package session

import (
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Status 表示认证会话的生命周期状态。
type Status string

const (
	// StatusActive 表示会话仍然有效。
	StatusActive Status = "active"
	// StatusRevoked 表示会话已被主动撤销。
	StatusRevoked Status = "revoked"
	// StatusExpired 表示会话已自然过期。
	StatusExpired Status = "expired"
)

// Session 表示一次登录会话。
type Session struct {
	SessionID string

	// —— 身份信息 —— //
	UserID          meta.ID // 用户ID
	LoginIdentityID meta.ID // 登录身份ID
	TenantID        meta.ID // 租户ID

	// —— 认证信息 —— //
	AuthContext  authentication.AuthenticationContext
	TokenContext authentication.TokenContext

	// —— 状态信息 —— //
	Status       Status     // 状态
	CreatedAt    time.Time  // 创建时间
	ExpiresAt    time.Time  // 过期时间
	RevokedAt    *time.Time // 撤销时间
	RevokeReason string     // 撤销原因
	RevokedBy    string     // 撤销者
}

// NewWithContexts 创建以强类型认证上下文和令牌上下文为权威来源的会话。
func NewWithContexts(sessionID string, userID, loginIdentityID, tenantID meta.ID, authContext authentication.AuthenticationContext, tokenContext authentication.TokenContext, expiresAt time.Time) *Session {
	now := time.Now()
	return &Session{
		SessionID: sessionID, UserID: userID, LoginIdentityID: loginIdentityID, TenantID: tenantID,
		AuthContext: authContext.Clone(), TokenContext: tokenContext.Clone(),
		Status: StatusActive, CreatedAt: now, ExpiresAt: expiresAt,
	}
}

// IsActive 返回会话是否仍处于可用状态。
func (s *Session) IsActive() bool {
	if s == nil {
		return false
	}
	if s.IsExpired() {
		return false
	}
	return s.Status == StatusActive
}

// IsExpired 返回会话是否已自然过期。
func (s *Session) IsExpired() bool {
	if s == nil {
		return true
	}
	return time.Now().After(s.ExpiresAt)
}

// RemainingTTL 返回当前会话剩余 TTL。
func (s *Session) RemainingTTL() time.Duration {
	if s == nil || s.IsExpired() {
		return 0
	}
	return time.Until(s.ExpiresAt)
}

// Revoke 将会话置为 revoked。
func (s *Session) Revoke(reason, revokedBy string) {
	if s == nil {
		return
	}
	now := time.Now()
	s.Status = StatusRevoked
	s.RevokedAt = &now
	s.RevokeReason = reason
	s.RevokedBy = revokedBy
}

// Extend 延长会话过期时间，并在必要时把已自然过期的会话重新拉回 active。
func (s *Session) Extend(expiresAt time.Time) {
	if s == nil {
		return
	}
	s.ExpiresAt = expiresAt
	if s.Status == StatusExpired {
		s.Status = StatusActive
	}
}
