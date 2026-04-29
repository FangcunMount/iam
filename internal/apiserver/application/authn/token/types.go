package token

import (
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// TokenType 表示 IAM 内部令牌用途。
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeService TokenType = "service"
)

// Token 是应用层令牌结果模型；具体编码格式由 infra token adapter 决定。
type Token struct {
	ID            string
	Type          TokenType
	Value         string
	Subject       string
	SessionID     string
	UserID        meta.ID
	AccountID     meta.ID
	TenantID      meta.ID
	Audience      []string
	Attributes    map[string]string
	AMR           []string
	SessionClaims map[string]string
	IssuedAt      time.Time
	ExpiresAt     time.Time
}

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

func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

func (t *Token) RemainingDuration() time.Duration {
	if t.IsExpired() {
		return 0
	}
	return time.Until(t.ExpiresAt)
}

type TokenPair struct {
	AccessToken  *Token
	RefreshToken *Token
}

func NewTokenPair(accessToken, refreshToken *Token) *TokenPair {
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}

// TokenClaims 是应用层验签结果，不暴露 JWT Header/Signature 等编码细节。
type TokenClaims struct {
	TokenID    string
	TokenType  TokenType
	Subject    string
	SessionID  string
	UserID     meta.ID
	AccountID  meta.ID
	TenantID   meta.ID
	Issuer     string
	Audience   []string
	Attributes map[string]string
	AMR        []string
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

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
