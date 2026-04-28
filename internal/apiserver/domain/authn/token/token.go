package token

import (
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// TokenType 令牌类型
type TokenType string

const (
	// TokenTypeAccess 访问令牌
	TokenTypeAccess TokenType = "access"
	// TokenTypeRefresh 刷新令牌
	TokenTypeRefresh TokenType = "refresh"
	// TokenTypeService 服务间访问令牌
	TokenTypeService TokenType = "service"
)

// Token 令牌值对象
type Token struct {
	ID         string    // 令牌唯一标识（用于撤销）
	Type       TokenType // 令牌类型
	Value      string    // 令牌值（JWT 字符串或 UUID）
	Subject    string    // JWT sub，服务令牌或访问令牌的主体
	SessionID  string    // 认证会话 ID；服务令牌为空
	UserID     meta.ID   // 关联的用户 ID
	AccountID  meta.ID
	TenantID   meta.ID
	Audience   []string          // JWT audience
	Attributes map[string]string // 附加属性（主要用于服务令牌）
	// AMR 认证方法引用；访问令牌在领域对象中可选，刷新令牌会持久化以便轮换 access 时保留。
	AMR []string
	// SessionClaims Principal.Claims 的字符串快照；仅刷新令牌持久化，用于再次签发 access。
	SessionClaims map[string]string
	IssuedAt      time.Time // 颁发时间
	ExpiresAt     time.Time // 过期时间
}

// NewAccessToken 创建访问令牌
func NewAccessToken(id, value, sessionID string, userID meta.ID, accountID meta.ID, tenantID meta.ID, expiresIn time.Duration) *Token {
	now := time.Now()
	return &Token{
		ID:        id,
		Type:      TokenTypeAccess,
		Value:     value,
		Subject:   userID.String(),
		SessionID: sessionID,
		UserID:    userID,
		AccountID: accountID,
		TenantID:  tenantID,
		IssuedAt:  now,
		ExpiresAt: now.Add(expiresIn),
	}
}

// NewServiceToken 创建服务间访问令牌。
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
func NewRefreshToken(id, value, sessionID string, userID meta.ID, accountID meta.ID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresIn time.Duration) *Token {
	now := time.Now()
	return &Token{
		ID:            id,
		Type:          TokenTypeRefresh,
		Value:         value,
		SessionID:     sessionID,
		UserID:        userID,
		AccountID:     accountID,
		TenantID:      tenantID,
		AMR:           cloneStrings(amr),
		SessionClaims: cloneStringMap(sessionClaims),
		IssuedAt:      now,
		ExpiresAt:     now.Add(expiresIn),
	}
}

// IsExpired 检查令牌是否过期
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// RemainingDuration 返回令牌剩余有效时长
func (t *Token) RemainingDuration() time.Duration {
	if t.IsExpired() {
		return 0
	}
	return time.Until(t.ExpiresAt)
}

// TokenPair 令牌对（访问令牌 + 刷新令牌）
type TokenPair struct {
	AccessToken  *Token
	RefreshToken *Token
}

// NewTokenPair 创建令牌对
func NewTokenPair(accessToken, refreshToken *Token) *TokenPair {
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}

// TokenClaims 令牌声明（从 JWT 解析出来的信息）
type TokenClaims struct {
	TokenID    string // 令牌 ID
	TokenType  TokenType
	Subject    string
	SessionID  string
	UserID     meta.ID // 用户 ID
	AccountID  meta.ID
	TenantID   meta.ID
	Issuer     string
	Audience   []string
	Attributes map[string]string
	AMR        []string  // RFC 8693 / OIDC amr
	IssuedAt   time.Time // 颁发时间
	ExpiresAt  time.Time // 过期时间
}

// NewTokenClaims 创建令牌声明
func NewTokenClaims(tokenType TokenType, tokenID, subject, sessionID string, userID meta.ID, accountID meta.ID, tenantID meta.ID, issuer string, audience []string, attributes map[string]string, amr []string, issuedAt, expiresAt time.Time) *TokenClaims {
	return &TokenClaims{
		TokenID:    tokenID,
		TokenType:  tokenType,
		Subject:    subject,
		SessionID:  sessionID,
		UserID:     userID,
		AccountID:  accountID,
		TenantID:   tenantID,
		Issuer:     issuer,
		Audience:   cloneStrings(audience),
		Attributes: cloneStringMap(attributes),
		AMR:        cloneStrings(amr),
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
	}
}

// IsExpired 检查令牌声明是否过期
func (c *TokenClaims) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
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
