// Package jwt JWT 令牌生成器实现
package jwt

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// Generator JWT 令牌生成器（使用 JWKS 的 RSA 密钥签名）
type Generator struct {
	issuer              string                  // 颁发者
	accessTokenAudience []string                // Access Token audience
	keyMgmt             jwks.Manager            // 密钥管理服务
	privKeyResolver     jwks.PrivateKeyResolver // 私钥解析器
}

// 确保 Generator 实现了 token.TokenGenerator 接口
var _ token.TokenGenerator = (*Generator)(nil)

// NewGenerator 创建 JWT 生成器
func NewGenerator(
	issuer string,
	accessTokenAudience []string,
	keyMgmt jwks.Manager,
	privKeyResolver jwks.PrivateKeyResolver,
) *Generator {
	if issuer == "" {
		issuer = "https://iam.fangcunmount.cn"
	}
	if len(accessTokenAudience) == 0 {
		accessTokenAudience = []string{"qs-api", "collection-api"}
	}
	return &Generator{
		issuer:              issuer,
		accessTokenAudience: cloneStrings(accessTokenAudience),
		keyMgmt:             keyMgmt,
		privKeyResolver:     privKeyResolver,
	}
}

// CustomClaims 自定义 JWT Claims
type CustomClaims struct {
	TokenType  string            `json:"token_type,omitempty"`
	SessionID  string            `json:"sid,omitempty"`
	UserID     string            `json:"user_id,omitempty"`
	AccountID  string            `json:"account_id,omitempty"`
	TenantID   string            `json:"tenant_id,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	AMR        []string          `json:"amr,omitempty"`
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成访问令牌（JWT）
// 使用 JWKS 中的活跃 RSA 密钥进行签名
func (g *Generator) GenerateAccessToken(ctx context.Context, principal *authentication.Principal, expiresIn time.Duration) (*domain.Token, error) {
	l := logger.L(ctx)
	l.Debugw("GenerateAccessToken", "principal", fmt.Sprintf("%+v", principal), "expiresIn", expiresIn)
	now := time.Now()
	tokenID := uuid.NewString()

	attr := authentication.FlattenClaimsForJWT(principal.Claims)
	claims := CustomClaims{
		TokenType:  string(domain.TokenTypeAccess),
		SessionID:  principal.SessionID,
		UserID:     principal.UserID.String(),
		AccountID:  principal.AccountID.String(),
		TenantID:   principal.TenantID.String(),
		Attributes: cloneStringMap(attr),
		AMR:        cloneStrings(principal.AMR),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Subject:   principal.UserID.String(),
			Issuer:    g.issuer,
			Audience:  jwt.ClaimStrings(cloneStrings(g.accessTokenAudience)),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	l.Debugw("GenerateAccessToken", "claims", claims)

	tokenString, err := g.signClaims(ctx, claims)
	if err != nil {
		l.Errorw("GenerateAccessToken", "error", err)
		return nil, err
	}

	l.Debugw("GenerateAccessToken", "tokenID", tokenID, "tokenString", tokenString, "principal", fmt.Sprintf("%+v", principal), "expiresIn", expiresIn)
	token := domain.NewAccessToken(
		tokenID,
		tokenString,
		principal.SessionID,
		principal.UserID,
		principal.AccountID,
		principal.TenantID,
		expiresIn,
	)
	l.Debugw("GenerateAccessToken", "token", token)
	return token, nil
}

// GenerateServiceToken 生成服务间访问令牌（JWT）。
func (g *Generator) GenerateServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*domain.Token, error) {
	now := time.Now()
	tokenID := uuid.NewString()

	claims := CustomClaims{
		TokenType:  string(domain.TokenTypeService),
		Attributes: cloneStringMap(attributes),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Subject:   subject,
			Issuer:    g.issuer,
			Audience:  jwt.ClaimStrings(cloneStrings(audience)),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	tokenString, err := g.signClaims(ctx, claims)
	if err != nil {
		return nil, err
	}

	return domain.NewServiceToken(tokenID, tokenString, subject, audience, attributes, expiresIn), nil
}

// ParseAccessToken 解析访问令牌
// 使用 JWKS 公钥验证 RSA 签名
func (g *Generator) ParseAccessToken(ctx context.Context, tokenValue string) (*domain.TokenClaims, error) {
	// 解析 token（不验证签名，先提取 kid）
	token, err := jwt.ParseWithClaims(tokenValue, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法是 RSA
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// 从 header 获取 kid
		kidInterface, ok := token.Header["kid"]
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}
		kid, ok := kidInterface.(string)
		if !ok {
			return nil, fmt.Errorf("invalid kid type in token header")
		}

		// 从 JWKS 获取密钥
		key, err := g.keyMgmt.GetKeyByKid(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get key %s: %w", kid, err)
		}

		// 从 JWK 解析 RSA 公钥用于验签
		if key == nil {
			return nil, fmt.Errorf("key not found for kid %s", kid)
		}

		if key.JWK.Kty != "RSA" {
			return nil, fmt.Errorf("unsupported key kty for verification: %s", key.JWK.Kty)
		}

		if key.JWK.N == nil || key.JWK.E == nil {
			return nil, fmt.Errorf("missing RSA parameters in JWK for kid %s", kid)
		}

		// n and e are base64url encoded (no padding)
		nBytes, err := base64.RawURLEncoding.DecodeString(*key.JWK.N)
		if err != nil {
			return nil, fmt.Errorf("failed to base64url-decode n for kid %s: %w", kid, err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(*key.JWK.E)
		if err != nil {
			return nil, fmt.Errorf("failed to base64url-decode e for kid %s: %w", kid, err)
		}

		n := new(big.Int).SetBytes(nBytes)

		// convert exponent bytes to int (big-endian)
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}
		if e == 0 {
			return nil, fmt.Errorf("invalid exponent parsed for kid %s", kid)
		}

		pub := &rsa.PublicKey{N: n, E: e}
		return pub, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// 提取 claims
	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// 转换为领域模型
	userID := parseStringID(claims.UserID)
	accountID := parseStringID(claims.AccountID)
	tenantID := parseStringID(claims.TenantID)
	tokenType := domain.TokenType(claims.TokenType)
	if tokenType == "" {
		tokenType = domain.TokenTypeAccess
	}
	return domain.NewTokenClaims(
		tokenType,
		claims.ID,
		claims.Subject,
		claims.SessionID,
		userID,
		accountID,
		tenantID,
		claims.Issuer,
		[]string(claims.Audience),
		claims.Attributes,
		claims.AMR,
		numericDateTime(claims.IssuedAt),
		numericDateTime(claims.ExpiresAt),
	), nil
}

func (g *Generator) signClaims(ctx context.Context, claims CustomClaims) (string, error) {
	activeKey, err := g.keyMgmt.GetActiveKey(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get active key: %w", err)
	}

	privKey, err := g.privKeyResolver.ResolveSigningKey(ctx, activeKey.Kid, activeKey.JWK.Alg)
	if err != nil {
		return "", fmt.Errorf("failed to resolve private key: %w", err)
	}

	rsaPrivKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("expected RSA private key, got %T", privKey)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = activeKey.Kid

	tokenString, err := token.SignedString(rsaPrivKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
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

func parseStringID(raw string) meta.ID {
	if raw == "" {
		return meta.FromUint64(0)
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return meta.FromUint64(0)
	}
	return meta.FromUint64(value)
}

func numericDateTime(v *jwt.NumericDate) time.Time {
	if v == nil {
		return time.Time{}
	}
	return v.Time
}
