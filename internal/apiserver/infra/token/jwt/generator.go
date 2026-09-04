// Package jwt implements IAM access/service token encoding with JWS compact JWT.
package jwt

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/logger"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/authnclaims"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	pkgauth "github.com/FangcunMount/iam/v3/pkg/auth"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type SigningKey struct {
	Kid        string
	Algorithm  string
	PrivateKey *rsa.PrivateKey
}

type VerificationKey struct {
	Kid       string
	Algorithm string
	PublicKey *rsa.PublicKey
}

// SigningKeySource 签名密钥源
type SigningKeySource interface {
	ActiveSigningKey(ctx context.Context) (*SigningKey, error)
	VerificationKey(ctx context.Context, kid string) (*VerificationKey, error)
}

// Generator JWT 生成器
type Generator struct {
	issuer              string           // 签发者
	accessTokenAudience []string         // 访问令牌受众
	keySource           SigningKeySource // 签名密钥源
}

// 实现 AccessTokenCodec 接口
var _ tokendomain.AccessTokenCodec = (*Generator)(nil)

// 创建新的 JWT 生成器
func NewGenerator(
	issuer string,
	accessTokenAudience []string,
	keySource SigningKeySource,
) *Generator {
	if issuer == "" {
		issuer = "https://iam.fangcunmount.cn"
	}
	// 如果访问令牌受众为空，则使用默认值
	if len(accessTokenAudience) == 0 {
		accessTokenAudience = []string{"qs-api", "collection-api"}
	}
	return &Generator{
		issuer:              issuer,
		accessTokenAudience: cloneStrings(accessTokenAudience),
		keySource:           keySource,
	}
}

// jwtPayloadClaims 是 JWT Payload 的 wire model，不向领域层泄漏。
type jwtPayloadClaims struct {
	TokenType       string            `json:"token_type,omitempty"`
	SessionID       string            `json:"sid,omitempty"`
	UserID          string            `json:"user_id,omitempty"`
	LoginIdentityID string            `json:"login_identity_id,omitempty"`
	OrgID           string            `json:"org_id,omitempty"`
	TenantID        string            `json:"tenant_id,omitempty"`
	AuthTime        int64             `json:"auth_time,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	AMR             []string          `json:"amr,omitempty"`
	jwtv4.RegisteredClaims
}

// IssueAccessToken 颁发访问令牌
func (g *Generator) IssueAccessToken(ctx context.Context, subject *tokendomain.AccessTokenSubject, expiresIn time.Duration) (*tokendomain.AccessToken, error) {
	// 只记录非敏感标识；subject 中可能包含第三方身份和业务属性。
	l := logger.L(ctx)
	l.Debugw("IssueAccessToken", "user_id", subject.UserID.String(), "session_id", subject.SessionID, "expires_in", expiresIn)

	// 准备令牌数据
	now := time.Now()
	tokenID := uuid.NewString()
	loginIdentityID := subject.LoginIdentityID

	// 创建 JWT 声明：adapter 只序列化领域投影结果，不再从任意 Claims/Realm 推断授权域。
	orgID := strings.TrimSpace(subject.OrgID)
	tenantDomain := strings.TrimSpace(subject.TenantDomain)
	attributes := cloneStringMap(subject.Attributes)
	authTimeUnix := int64(0)
	if !subject.AuthenticatedAt.IsZero() {
		authTimeUnix = subject.AuthenticatedAt.UTC().Unix()
		if attributes == nil {
			attributes = map[string]string{}
		}
		// 迁移窗口：同时写入 attributes.auth_time，供旧消费者双读。
		attributes["auth_time"] = subject.AuthenticatedAt.UTC().Format(time.RFC3339)
	}
	claims := jwtPayloadClaims{
		TokenType:       string(tokendomain.TokenTypeAccess),
		SessionID:       subject.SessionID,
		UserID:          subject.UserID.String(),
		LoginIdentityID: loginIdentityID.String(),
		OrgID:           orgID,
		TenantID:        tenantDomain,
		AuthTime:        authTimeUnix,
		Attributes:      attributes,
		AMR:             cloneStrings(subject.AMR),
		RegisteredClaims: jwtv4.RegisteredClaims{
			ID:        tokenID,
			Subject:   subject.UserID.String(),
			Issuer:    g.issuer,
			Audience:  jwtv4.ClaimStrings(cloneStrings(g.accessTokenAudience)),
			IssuedAt:  jwtv4.NewNumericDate(now),
			ExpiresAt: jwtv4.NewNumericDate(now.Add(expiresIn)),
			NotBefore: jwtv4.NewNumericDate(now),
		},
	}

	// 签名 JWT
	tokenString, err := g.signClaims(ctx, claims)
	if err != nil {
		return nil, err
	}

	// 创建访问令牌
	token := tokendomain.NewAccessToken(
		tokenID,
		tokenString,
		subject.SessionID,
		subject.UserID,
		loginIdentityID,
		subject.TenantID,
		expiresIn,
	)
	// 设置登录身份 ID
	token.LoginIdentityID = loginIdentityID
	return token, nil
}

// IssueServiceToken 颁发服务令牌
func (g *Generator) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*tokendomain.ServiceToken, error) {
	// 获取当前时间
	now := time.Now()
	// 生成令牌 ID
	tokenID := uuid.NewString()
	allowedAttributes := authnclaims.EncodeServiceAttributes(attributes)
	// 创建 JWT 声明
	claims := jwtPayloadClaims{
		TokenType:  string(tokendomain.TokenTypeService),
		Attributes: cloneStringMap(allowedAttributes),
		RegisteredClaims: jwtv4.RegisteredClaims{
			ID:        tokenID,
			Subject:   subject,
			Issuer:    g.issuer,
			Audience:  jwtv4.ClaimStrings(cloneStrings(audience)),
			IssuedAt:  jwtv4.NewNumericDate(now),
			ExpiresAt: jwtv4.NewNumericDate(now.Add(expiresIn)),
			NotBefore: jwtv4.NewNumericDate(now),
		},
	}
	// 签名 JWT
	tokenString, err := g.signClaims(ctx, claims)
	if err != nil {
		return nil, err
	}
	// 创建服务令牌
	return tokendomain.NewServiceToken(tokenID, tokenString, subject, audience, allowedAttributes, expiresIn), nil
}

// VerifyAccessToken 验证访问令牌

func (g *Generator) VerifyAccessToken(ctx context.Context, tokenValue string) (*tokendomain.TokenClaims, error) {
	// 解析 JWT
	parsed, err := jwtv4.ParseWithClaims(tokenValue, &jwtPayloadClaims{},
		func(token *jwtv4.Token) (interface{}, error) {
			headerAlgorithm, ok := token.Header["alg"].(string)
			if !ok || headerAlgorithm != pkgauth.TokenProfileAlgorithm || token.Method.Alg() != pkgauth.TokenProfileAlgorithm {
				return nil, fmt.Errorf("unexpected signing algorithm: %v", token.Header["alg"])
			}
			// 获取签名密钥 ID
			kidInterface, ok := token.Header["kid"]
			if !ok {
				return nil, fmt.Errorf("missing kid in token header")
			}
			kid, ok := kidInterface.(string)
			if !ok {
				return nil, fmt.Errorf("invalid kid type in token header")
			}
			if kid == "" {
				return nil, fmt.Errorf("empty kid in token header")
			}

			// 获取签名密钥
			key, err := g.keySource.VerificationKey(ctx, kid)
			if err != nil {
				return nil, fmt.Errorf("failed to get key %s: %w", kid, err)
			}
			if key == nil {
				return nil, fmt.Errorf("key not found for kid %s", kid)
			}
			if key.Kid != kid {
				return nil, fmt.Errorf("verification key kid mismatch: header=%s key=%s", kid, key.Kid)
			}
			if key.Algorithm != headerAlgorithm || key.Algorithm != pkgauth.TokenProfileAlgorithm {
				return nil, fmt.Errorf("verification algorithm mismatch: header=%s key=%s", headerAlgorithm, key.Algorithm)
			}
			if key.PublicKey == nil {
				return nil, fmt.Errorf("verification key is nil for kid %s", kid)
			}

			// 返回签名密钥
			return key.PublicKey, nil
		},
		jwtv4.WithValidMethods([]string{pkgauth.TokenProfileAlgorithm}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// 解析 JWT 声明
	claims, ok := parsed.Claims.(*jwtPayloadClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	if claims.Issuer != g.issuer {
		return nil, fmt.Errorf("unexpected token issuer: %q", claims.Issuer)
	}

	// 获取令牌类型。兼容窗口内缺失 token_type 仍按 access 处理，并记录有界指标。
	tokenType := tokendomain.TokenType(claims.TokenType)
	switch tokenType {
	case "":
		missingTokenTypeTotal.Inc()
		tokenType = tokendomain.TokenTypeAccess
	case tokendomain.TokenTypeAccess, tokendomain.TokenTypeService:
	default:
		return nil, fmt.Errorf("unsupported token_type: %q", claims.TokenType)
	}

	// 解析登录身份 ID
	loginIdentityID := parseStringID(claims.LoginIdentityID)
	// 解析组织 ID
	orgID := parseStringID(claims.OrgID)
	// 解析租户 ID
	tenantDomain, _ := parseTenantIDClaim(claims.TenantID)
	attributes := cloneStringMap(claims.Attributes)
	authTime := time.Time{}
	if claims.AuthTime > 0 {
		authTime = time.Unix(claims.AuthTime, 0).UTC()
		if attributes == nil {
			attributes = map[string]string{}
		}
		if attributes["auth_time"] == "" {
			attributes["auth_time"] = authTime.Format(time.RFC3339)
		}
	} else if raw := strings.TrimSpace(attributes["auth_time"]); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			legacyAttributeAuthTimeFallbackTotal.Inc()
			authTime = parsed.UTC()
		}
	}
	verified := tokendomain.VerifiedTokenClaims{
		TokenID: claims.ID, TokenType: tokenType, Subject: claims.Subject, SessionID: claims.SessionID,
		UserID: parseStringID(claims.UserID), LoginIdentityID: loginIdentityID,
		OrgID: orgID, TenantDomain: tenantDomain, Issuer: claims.Issuer,
		Audience: []string(claims.Audience), Attributes: attributes, AMR: claims.AMR,
		AuthenticatedAt: authTime, IssuedAt: numericDateTime(claims.IssuedAt),
		NotBefore: numericDateTime(claims.NotBefore), ExpiresAt: numericDateTime(claims.ExpiresAt),
	}
	if tokenType == tokendomain.TokenTypeService {
		return tokendomain.NewVerifiedServiceClaims(verified)
	}
	return tokendomain.NewVerifiedUserTokenClaims(verified)
}

// signClaims 签名 JWT 声明

func (g *Generator) signClaims(ctx context.Context, claims jwtPayloadClaims) (string, error) {
	// 获取活动签名密钥
	key, err := g.keySource.ActiveSigningKey(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to resolve active signing key: %w", err)
	}
	if key == nil || key.PrivateKey == nil {
		return "", fmt.Errorf("active signing key is nil")
	}
	if key.Kid == "" {
		return "", fmt.Errorf("active signing key kid is empty")
	}
	if key.Algorithm != pkgauth.TokenProfileAlgorithm {
		return "", fmt.Errorf("active signing key algorithm %q is not allowed", key.Algorithm)
	}
	// 创建 JWT 令牌
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodRS256, claims)
	// 设置令牌类型
	token.Header["typ"] = headerTypeJWT
	// 设置签名密钥 ID
	token.Header["kid"] = key.Kid
	// 签名 JWT 令牌
	return token.SignedString(key.PrivateKey)
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

// parseStringID 解析字符串 ID
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

// numericDateTime 解析数字日期时间
func numericDateTime(v *jwtv4.NumericDate) time.Time {
	if v == nil {
		return time.Time{}
	}
	return v.Time
}
