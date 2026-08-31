package token

import (
	"time"

	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Token 是向 application 调用方返回的令牌 DTO；领域模型位于 domain/authn/token。
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
	return tokenFromAccess(tokendomain.NewAccessToken(id, value, sessionID, userID, loginIdentityID, tenantID, expiresIn))
}

// NewServiceToken 创建服务令牌
func NewServiceToken(id, value, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) *Token {
	return tokenFromService(tokendomain.NewServiceToken(id, value, subject, audience, attributes, expiresIn))
}

// NewRefreshToken 创建相对当前时间过期的刷新令牌。
// 该构造器用于需要表达 TTL 的测试和调用方；生产签发路径使用显式过期时间构造器。
func NewRefreshToken(id, value, sessionID string, userID meta.ID, loginIdentityID meta.ID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresIn time.Duration) *Token {
	return tokenFromRefresh(tokendomain.NewRefreshToken(id, value, sessionID, userID, loginIdentityID, tenantID, amr, sessionClaims, expiresIn))
}

// NewRefreshTokenWithExpiry 创建指定过期时间的刷新令牌。
func NewRefreshTokenWithExpiry(id, value, sessionID string, userID meta.ID, loginIdentityID meta.ID, tenantID meta.ID, amr []string, sessionClaims map[string]string, expiresAt time.Time) *Token {
	return tokenFromRefresh(tokendomain.NewRefreshTokenWithExpiry(id, value, sessionID, userID, loginIdentityID, tenantID, amr, sessionClaims, expiresAt))
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

// NewTokenClaims 保留 application 调用契约并委托领域构造器。
func NewTokenClaims(tokenType TokenType, tokenID, subject, sessionID string, userID, loginIdentityID, orgID meta.ID, tenantDomain, issuer string, audience []string, attributes map[string]string, amr []string, issuedAt, expiresAt time.Time) *TokenClaims {
	return tokendomain.NewTokenClaims(tokenType, tokenID, subject, sessionID, userID, loginIdentityID, orgID, tenantDomain, issuer, audience, attributes, amr, issuedAt, expiresAt)
}

func tokenPairFromDomain(set *tokendomain.UserTokenSet) *TokenPair {
	if set == nil {
		return nil
	}
	return NewTokenPair(tokenFromAccess(set.AccessToken), tokenFromRefresh(set.RefreshToken))
}

func tokenFromAccess(token *tokendomain.AccessToken) *Token {
	if token == nil {
		return nil
	}
	return &Token{
		ID: token.ID, Type: TokenTypeAccess, Value: token.Value, Subject: token.Subject,
		SessionID: token.SessionID, UserID: token.UserID, LoginIdentityID: token.LoginIdentityID,
		TenantID: token.TenantID, AuthMethod: token.AuthMethod, Realm: token.Realm,
		IssuedAt: token.IssuedAt, ExpiresAt: token.ExpiresAt,
	}
}

func tokenFromRefresh(token *tokendomain.RefreshToken) *Token {
	if token == nil {
		return nil
	}
	return &Token{
		ID: token.ID, Type: TokenTypeRefresh, Value: token.Value, SessionID: token.SessionID,
		UserID: token.UserID, LoginIdentityID: token.LoginIdentityID, TenantID: token.TenantID,
		AuthMethod: token.AuthMethod, Realm: token.Realm, AMR: cloneStrings(token.AMR),
		SessionClaims: cloneStringMap(token.SessionClaims), IssuedAt: token.IssuedAt, ExpiresAt: token.ExpiresAt,
	}
}

func tokenFromService(token *tokendomain.ServiceToken) *Token {
	if token == nil {
		return nil
	}
	return &Token{
		ID: token.ID, Type: TokenTypeService, Value: token.Value, Subject: token.Subject,
		Audience: cloneStrings(token.Audience), Attributes: cloneStringMap(token.Attributes),
		IssuedAt: token.IssuedAt, ExpiresAt: token.ExpiresAt,
	}
}
