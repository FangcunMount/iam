package session

import (
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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
	SessionID     string
	UserID        meta.ID
	AccountID     meta.ID
	TenantID      meta.ID
	Status        Status
	AMR           []string
	SessionClaims map[string]string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	RevokeReason  string
	RevokedBy     string
}

// New 创建一个新的活跃会话。
func New(sessionID string, userID, accountID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresAt time.Time) *Session {
	now := time.Now()
	return &Session{
		SessionID:     sessionID,
		UserID:        userID,
		AccountID:     accountID,
		TenantID:      tenantID,
		Status:        StatusActive,
		AMR:           cloneStrings(amr),
		SessionClaims: cloneStringMap(sessionClaims),
		CreatedAt:     now,
		ExpiresAt:     expiresAt,
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

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
