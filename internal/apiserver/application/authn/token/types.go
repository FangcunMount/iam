package token

import (
	"time"

	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SessionManager 是 token 用例依赖的会话领域协作者。
type SessionManager = sessiondomain.Manager

// SubjectAccessEvaluator 是 token 用例依赖的主体访问状态领域协作者。
type SubjectAccessEvaluator = sessiondomain.SubjectAccessEvaluator

// TokenType 表示 IAM 内部令牌用途。
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"  // 访问令牌
	TokenTypeRefresh TokenType = "refresh" // 刷新令牌
	TokenTypeService TokenType = "service" // 服务令牌
)

// Token 是应用层令牌结果模型；具体编码格式由 infra token adapter 决定。
type Token struct {
	ID              string
	Type            TokenType
	Value           string
	Subject         string
	SessionID       string
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantID        meta.ID
	AuthMethod      string
	Realm           string
	Audience        []string
	Attributes      map[string]string
	AMR             []string
	SessionClaims   map[string]string
	IssuedAt        time.Time
	ExpiresAt       time.Time
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
		IssuedAt:        now,
		ExpiresAt:       now.Add(expiresIn),
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

// TokenClaims 是应用层验签结果，不暴露 JWT Header/Signature 等编码细节
type TokenClaims struct {
	TokenID         string
	TokenType       TokenType
	Subject         string
	SessionID       string
	UserID          meta.ID
	LoginIdentityID meta.ID
	// OrgID 来自 JWT org_id claim 的业务上下文透传，非 IAM 身份域字段。
	OrgID meta.ID
	// TenantDomain IAM 授权域（JWT tenant_id claim，string，如 fangcun）。
	TenantDomain string
	// TenantID Deprecated: 历史误将数值 org 写入 tenant_id；新 token 请读 TenantDomain。
	TenantID meta.ID
	AuthMethod      string
	Realm           string
	Issuer          string
	Audience        []string
	Attributes      map[string]string
	AMR             []string
	IssuedAt        time.Time
	ExpiresAt       time.Time
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

// cloneStrings 克隆字符串切片
func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// cloneStringMap 克隆字符串映射
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

// Principal 是 access token 编码所需的应用层身份快照（IAM 身份域，不含 org）。
// 授权域见 Claims["tenant_domain"] 或 Realm；业务 org 见 Claims["org_id"]。
type Principal struct {
	UserID          meta.ID
	LoginIdentityID meta.ID
	SessionID       string
	AuthMethod      string
	Realm           string
	AMR             []string
	Claims          map[string]any
}
