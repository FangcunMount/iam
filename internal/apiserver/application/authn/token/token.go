package token

import (
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Token 是应用层令牌结果模型。
type Token struct {
	ID string // 令牌ID

	// --- 令牌主体信息 ---
	Type    TokenType // 令牌类型
	Value   string    // 令牌值
	Subject string    // 令牌主体

	// --- 令牌主体会话信息 ---
	SessionID       string  // 会话ID
	UserID          meta.ID // 用户ID
	LoginIdentityID meta.ID // 登录身份ID
	TenantID        meta.ID // 租户ID

	// --- 令牌主体上下文信息 ---
	AuthMethod    string            // 认证方法
	Realm         string            // 认证域
	Audience      []string          // 受众
	Attributes    map[string]string // 属性
	AMR           []string          // 认证方法引用
	SessionClaims map[string]string // 会话声明
	IssuedAt      time.Time         // 颁发时间
	ExpiresAt     time.Time         // 过期时间
}

// NewAccessToken 创建访问令牌
func NewAccessToken(id, value, sessionID string, userID meta.ID, loginIdentityID meta.ID, tenantID meta.ID, expiresIn time.Duration) *Token {
	now := time.Now()
	return &Token{
		ID:              id,
		Type:            TokenTypeAccess,
		Value:           value,
		Subject:         userID.String(),
		SessionID:       sessionID,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
		TenantID:        tenantID,
		IssuedAt:        now,
		ExpiresAt:       now.Add(expiresIn),
	}
}

// NewServiceToken 创建服务令牌
func NewServiceToken(id, value, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) *Token {
	now := time.Now()
	return &Token{
		ID:         id,
		Type:       TokenTypeService,
		Value:      value,
		Subject:    subject,
		Audience:   cloneStrings(audience),
		Attributes: cloneStringMap(attributes),
		IssuedAt:   now,
		ExpiresAt:  now.Add(expiresIn),
	}
}

// NewRefreshToken 创建刷新令牌
func NewRefreshToken(id, value, sessionID string, userID meta.ID, loginIdentityID meta.ID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresIn time.Duration) *Token {
	now := time.Now()
	return newRefreshToken(id, value, sessionID, userID, loginIdentityID, tenantID, amr, sessionClaims, now, now.Add(expiresIn))
}

// NewRefreshTokenWithExpiry 创建指定过期时间的刷新令牌。
func NewRefreshTokenWithExpiry(id, value, sessionID string, userID meta.ID, loginIdentityID meta.ID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresAt time.Time) *Token {
	return newRefreshToken(id, value, sessionID, userID, loginIdentityID, tenantID, amr, sessionClaims, time.Now(), expiresAt)
}

// newRefreshToken 创建刷新令牌。
func newRefreshToken(id, value, sessionID string, userID meta.ID, loginIdentityID meta.ID, tenantID meta.ID, amr []string, sessionClaims map[string]string, issuedAt, expiresAt time.Time) *Token {
	return &Token{
		ID:              id,
		Type:            TokenTypeRefresh,
		Value:           value,
		SessionID:       sessionID,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
		TenantID:        tenantID,
		AMR:             cloneStrings(amr),
		SessionClaims:   cloneStringMap(sessionClaims),
		IssuedAt:        issuedAt,
		ExpiresAt:       expiresAt,
	}
}

// IsExpired 检查令牌是否已过期
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// RemainingDuration 返回令牌剩余时间
func (t *Token) RemainingDuration() time.Duration {
	if t.IsExpired() {
		return 0
	}
	return time.Until(t.ExpiresAt)
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  *Token // 访问令牌
	RefreshToken *Token // 刷新令牌
}

// NewTokenPair 创建令牌对
func NewTokenPair(accessToken, refreshToken *Token) *TokenPair {
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}

// TokenClaims 是应用层验签结果，不暴露 JWT Header/Signature 等编码细节
type TokenClaims struct {
	// --- 令牌元数据 ---
	TokenID   string    // 令牌ID
	TokenType TokenType // 令牌类型
	Subject   string    // 令牌主体

	// --- 令牌主体会话信息 ---
	SessionID string

	// --- 令牌主体用户信息 ---
	UserID          meta.ID // 用户ID
	LoginIdentityID meta.ID // 登录身份ID

	// --- 令牌主体租户信息 ---
	TenantDomain string  // 租户域
	TenantID     meta.ID // 租户ID
	OrgID        meta.ID // 组织ID（非 IAM 领域字段，用于业务上下文透传）

	// --- 令牌主体认证信息 ---
	AuthMethod string            // 认证方法（可选）
	Realm      string            // 认证域（可选）
	Issuer     string            // 签发者（可选）
	Audience   []string          // 受众（可选）
	Attributes map[string]string // 属性（可选）
	AMR        []string          // 认证方法引用（可选）
	IssuedAt   time.Time         // 颁发时间（可选）
	ExpiresAt  time.Time         // 过期时间（可选）
}

// NewTokenClaims 创建令牌声明
func NewTokenClaims(tokenType TokenType, tokenID, subject, sessionID string, userID meta.ID, loginIdentityID meta.ID, orgID meta.ID, tenantDomain string, issuer string, audience []string, attributes map[string]string, amr []string, issuedAt, expiresAt time.Time) *TokenClaims {
	return &TokenClaims{
		TokenID:         tokenID,
		TokenType:       tokenType,
		Subject:         subject,
		SessionID:       sessionID,
		UserID:          userID,
		LoginIdentityID: loginIdentityID,
		OrgID:           orgID,
		TenantDomain:    tenantDomain,
		Issuer:          issuer,
		Audience:        cloneStrings(audience),
		Attributes:      cloneStringMap(attributes),
		AMR:             cloneStrings(amr),
		IssuedAt:        issuedAt,
		ExpiresAt:       expiresAt,
	}
}

// IsExpired 检查令牌声明是否已过期
func (c *TokenClaims) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}
