package token

import (
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// TokenType 表示 IAM 令牌的领域用途。
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeService TokenType = "service"
)

// Token 是 AuthN 令牌概念族的共同契约。
// 具体不变量由 AccessToken、RefreshToken 和 ServiceToken 分别表达。
type Token interface {
	Kind() TokenType
	Metadata() TokenMetadata
}

// TokenMetadata 是三类令牌共享的身份与生命周期信息。
type TokenMetadata struct {
	// —— 身份信息 —— //
	ID    string // 令牌ID
	Value string // 令牌值

	// —— 生命周期信息 —— //
	IssuedAt  time.Time // 令牌颁发时间
	ExpiresAt time.Time // 令牌过期时间
}

// Metadata 返回令牌元数据的值副本。
func (m TokenMetadata) Metadata() TokenMetadata { return m }

// IsExpiredAt 返回令牌在指定时刻是否已过期。
func (m TokenMetadata) IsExpiredAt(now time.Time) bool {
	return now.After(m.ExpiresAt)
}

// IsExpired 返回令牌当前是否已过期。
func (m TokenMetadata) IsExpired() bool {
	return m.IsExpiredAt(time.Now())
}

// RemainingAt 返回令牌在指定时刻的剩余有效期。
func (m TokenMetadata) RemainingAt(now time.Time) time.Duration {
	if m.IsExpiredAt(now) {
		return 0
	}
	return m.ExpiresAt.Sub(now)
}

// RemainingDuration 返回令牌当前剩余有效期。
func (m TokenMetadata) RemainingDuration() time.Duration {
	return m.RemainingAt(time.Now())
}

// AccessToken 表示绑定用户认证上下文与 Session 的短期访问凭证。
type AccessToken struct {
	TokenMetadata

	// —— 主体信息 —— //
	Subject string // 令牌主题

	// —— 会话信息 —— //
	SessionID       string  // 会话ID
	UserID          meta.ID // 用户ID
	LoginIdentityID meta.ID // 登录身份ID
	TenantID        meta.ID // 租户ID

	// —— 认证信息 —— //
	AuthMethod string // 认证方法
	Realm      string // 认证域
}

func (*AccessToken) Kind() TokenType { return TokenTypeAccess }

// NewAccessToken 创建访问令牌。
func NewAccessToken(id, value, sessionID string, userID, loginIdentityID, tenantID meta.ID, expiresIn time.Duration) *AccessToken {
	now := time.Now()
	return &AccessToken{
		TokenMetadata:   TokenMetadata{ID: id, Value: value, IssuedAt: now, ExpiresAt: now.Add(expiresIn)},
		Subject:         userID.String(),
		SessionID:       sessionID,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
		TenantID:        tenantID,
	}
}

// RefreshToken 表示与认证 Session 绑定、可单次轮换的续期凭证。
type RefreshToken struct {
	TokenMetadata

	// —— 会话信息 —— //
	SessionID       string  // 会话ID
	UserID          meta.ID // 用户ID
	LoginIdentityID meta.ID // 登录身份ID
	TenantID        meta.ID // 租户ID

	// —— 认证信息 —— //
	AuthMethod    string            // 认证方法
	Realm         string            // 认证域
	AMR           []string          // 认证方法引用
	SessionClaims map[string]string // 认证声明
}

func (*RefreshToken) Kind() TokenType { return TokenTypeRefresh }

// NewRefreshToken 创建相对当前时间过期的刷新令牌。
func NewRefreshToken(id, value, sessionID string, userID, loginIdentityID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresIn time.Duration) *RefreshToken {
	now := time.Now()
	return newRefreshToken(id, value, sessionID, userID, loginIdentityID, tenantID, amr, sessionClaims, now, now.Add(expiresIn))
}

// NewRefreshTokenWithExpiry 创建指定过期时间的刷新令牌。
func NewRefreshTokenWithExpiry(id, value, sessionID string, userID, loginIdentityID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresAt time.Time) *RefreshToken {
	return newRefreshToken(id, value, sessionID, userID, loginIdentityID, tenantID, amr, sessionClaims, time.Now(), expiresAt)
}

func newRefreshToken(id, value, sessionID string, userID, loginIdentityID, tenantID meta.ID, amr []string, sessionClaims map[string]string, issuedAt, expiresAt time.Time) *RefreshToken {
	return &RefreshToken{
		TokenMetadata:   TokenMetadata{ID: id, Value: value, IssuedAt: issuedAt, ExpiresAt: expiresAt},
		SessionID:       sessionID,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
		TenantID:        tenantID,
		AMR:             cloneStrings(amr),
		SessionClaims:   cloneStringMap(sessionClaims),
	}
}

// ServiceToken 表示不绑定用户 Session 的服务间访问凭证。
type ServiceToken struct {
	TokenMetadata

	Subject    string
	Audience   []string
	Attributes map[string]string
}

func (*ServiceToken) Kind() TokenType { return TokenTypeService }

// NewServiceToken 创建服务令牌。
func NewServiceToken(id, value, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) *ServiceToken {
	now := time.Now()
	return &ServiceToken{
		TokenMetadata: TokenMetadata{ID: id, Value: value, IssuedAt: now, ExpiresAt: now.Add(expiresIn)},
		Subject:       subject,
		Audience:      cloneStrings(audience),
		Attributes:    cloneStringMap(attributes),
	}
}

// UserTokenSet 表示一次用户认证状态建立或续期产生的访问/刷新令牌集合。
type UserTokenSet struct {
	AccessToken  *AccessToken
	RefreshToken *RefreshToken
}

// NewUserTokenSet 创建用户令牌集合。
func NewUserTokenSet(accessToken *AccessToken, refreshToken *RefreshToken) *UserTokenSet {
	return &UserTokenSet{AccessToken: accessToken, RefreshToken: refreshToken}
}

// ConsumedRefreshToken 是旧刷新令牌成功轮换后留下的最小重放检测事实。
type ConsumedRefreshToken struct {
	SessionID string
	UserID    meta.ID
}

// TokenClaims 令牌声明
// 是验签后得到的令牌领域声明，不暴露 JWT Header/Signature。
// 令牌声明包含令牌的元数据、主体、认证、属性、时间等信息。
type TokenClaims struct {
	// —— 令牌元数据 —— //
	TokenID   string
	TokenType TokenType
	SessionID string
	Subject   string

	// —— 令牌主体 —— //
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantDomain    string
	TenantID        meta.ID
	OrgID           meta.ID

	// —— 令牌认证 —— //
	AuthMethod string
	Realm      string
	Issuer     string

	// —— 令牌属性 —— //
	Audience   []string
	Attributes map[string]string
	AMR        []string

	// —— 令牌时间 —— //
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// NewTokenClaims 创建验签后的令牌声明。
func NewTokenClaims(tokenType TokenType, tokenID, subject, sessionID string, userID, loginIdentityID, orgID meta.ID, tenantDomain, issuer string, audience []string, attributes map[string]string, amr []string, issuedAt, expiresAt time.Time) *TokenClaims {
	return &TokenClaims{
		TokenID: tokenID, TokenType: tokenType, Subject: subject, SessionID: sessionID,
		UserID: userID, LoginIdentityID: loginIdentityID, OrgID: orgID, TenantDomain: tenantDomain,
		Issuer: issuer, Audience: cloneStrings(audience), Attributes: cloneStringMap(attributes),
		AMR: cloneStrings(amr), IssuedAt: issuedAt, ExpiresAt: expiresAt,
	}
}

// IsExpiredAt 返回声明在指定时刻是否已过期。
func (c *TokenClaims) IsExpiredAt(now time.Time) bool {
	return c == nil || now.After(c.ExpiresAt)
}

// IsExpired 返回声明当前是否已过期。
func (c *TokenClaims) IsExpired() bool { return c.IsExpiredAt(time.Now()) }

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
	for key, value := range in {
		out[key] = value
	}
	return out
}
