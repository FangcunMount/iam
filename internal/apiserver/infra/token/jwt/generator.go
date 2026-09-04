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
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// SigningKeySource 签名密钥源
type SigningKeySource interface {
	ActiveSigningKey(ctx context.Context) (kid string, privateKey *rsa.PrivateKey, err error)
	VerificationKey(ctx context.Context, kid string) (*rsa.PublicKey, error)
}

// Generator JWT 生成器
type Generator struct {
	issuer              string              // 签发者
	accessTokenAudience []string            // 访问令牌受众
	keySource           SigningKeySource    // 签名密钥源
	attributeEncoder    jwtAttributeEncoder // 属性编码器
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
		attributeEncoder:    newJWTAttributeEncoder(),
	}
}

// CustomClaims 自定义 JWT 声明
type CustomClaims struct {
	TokenType       string            `json:"token_type,omitempty"`
	SessionID       string            `json:"sid,omitempty"`
	UserID          string            `json:"user_id,omitempty"`
	LoginIdentityID string            `json:"login_identity_id,omitempty"`
	OrgID           string            `json:"org_id,omitempty"`
	TenantID        string            `json:"tenant_id,omitempty"`
	AuthMethod      string            `json:"auth_method,omitempty"`
	Realm           string            `json:"realm,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	AMR             []string          `json:"amr,omitempty"`
	jwtv4.RegisteredClaims
}

// IssueAccessToken 颁发访问令牌
func (g *Generator) IssueAccessToken(ctx context.Context, subject *tokendomain.AccessTokenSubject, expiresIn time.Duration) (*tokendomain.AccessToken, error) {
	// 记录日志
	l := logger.L(ctx)
	l.Debugw("IssueAccessToken", "subject", fmt.Sprintf("%+v", subject), "expiresIn", expiresIn)

	// 准备令牌数据
	now := time.Now()
	tokenID := uuid.NewString()
	loginIdentityID := subject.LoginIdentityID
	authMethod := subject.AuthMethod
	realm := subject.Realm

	// 创建 JWT 声明
	claims := CustomClaims{
		TokenType:       string(tokendomain.TokenTypeAccess),
		SessionID:       subject.SessionID,
		UserID:          subject.UserID.String(),
		LoginIdentityID: loginIdentityID.String(),
		OrgID:           businessOrgIDClaim(subject.Claims),
		TenantID:        tenantDomainFromClaims(subject.Claims, subject.Realm),
		AuthMethod:      authMethod,
		Realm:           realm,
		Attributes:      cloneStringMap(g.attributeEncoder.EncodeAttributes(subject.Claims)),
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
	// 设置认证方法
	token.AuthMethod = authMethod
	// 设置认证域
	token.Realm = realm
	return token, nil
}

// IssueServiceToken 颁发服务令牌
func (g *Generator) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*tokendomain.ServiceToken, error) {
	// 获取当前时间
	now := time.Now()
	// 生成令牌 ID
	tokenID := uuid.NewString()
	// 创建 JWT 声明
	claims := CustomClaims{
		TokenType:  string(tokendomain.TokenTypeService),
		Attributes: cloneStringMap(attributes),
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
	return tokendomain.NewServiceToken(tokenID, tokenString, subject, audience, attributes, expiresIn), nil
}

// VerifyAccessToken 验证访问令牌

func (g *Generator) VerifyAccessToken(ctx context.Context, tokenValue string) (*tokendomain.TokenClaims, error) {
	// 解析 JWT
	parsed, err := jwtv4.ParseWithClaims(tokenValue, &CustomClaims{},
		func(token *jwtv4.Token) (interface{}, error) {
			// 如果签名方法不是 RSA，则返回错误
			if _, ok := token.Method.(*jwtv4.SigningMethodRSA); !ok {
				// 如果签名方法不是 RSA，则返回错误
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
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

			// 获取签名密钥
			key, err := g.keySource.VerificationKey(ctx, kid)
			if err != nil {
				return nil, fmt.Errorf("failed to get key %s: %w", kid, err)
			}
			if key == nil {
				return nil, fmt.Errorf("key not found for kid %s", kid)
			}

			// 返回签名密钥
			return key, nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// 解析 JWT 声明
	claims, ok := parsed.Claims.(*CustomClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// 获取令牌类型
	tokenType := tokendomain.TokenType(claims.TokenType)
	// 如果令牌类型为空，则设置为访问令牌类型
	if tokenType == "" {
		tokenType = tokendomain.TokenTypeAccess
	}

	// 解析登录身份 ID
	loginIdentityID := parseStringID(claims.LoginIdentityID)
	// 解析组织 ID
	orgID := parseStringID(claims.OrgID)
	// 解析租户 ID
	tenantDomain, _ := parseTenantIDClaim(claims.TenantID)
	// 创建令牌声明
	tokenClaims := tokendomain.NewTokenClaims(
		tokenType,
		claims.ID,
		claims.Subject,
		claims.SessionID,
		parseStringID(claims.UserID),
		loginIdentityID,
		orgID,
		tenantDomain,
		claims.Issuer,
		[]string(claims.Audience),
		claims.Attributes,
		claims.AMR,
		numericDateTime(claims.IssuedAt),
		numericDateTime(claims.ExpiresAt),
	)
	// 设置认证方法
	tokenClaims.AuthMethod = claims.AuthMethod
	// 设置认证域
	tokenClaims.Realm = claims.Realm
	// 返回令牌声明
	return tokenClaims, nil
}

// signClaims 签名 JWT 声明

func (g *Generator) signClaims(ctx context.Context, claims CustomClaims) (string, error) {
	// 获取活动签名密钥
	kid, rsaPrivKey, err := g.keySource.ActiveSigningKey(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to resolve active signing key: %w", err)
	}
	// 创建 JWT 令牌
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodRS256, claims)
	// 设置令牌类型
	token.Header["typ"] = headerTypeJWT
	// 设置签名密钥 ID
	token.Header["kid"] = kid
	// 签名 JWT 令牌
	return token.SignedString(rsaPrivKey)
}

// businessOrgIDClaim 从 Principal.Claims 读取业务侧提供的 org_id，IAM 不生成默认值。
func businessOrgIDClaim(claims map[string]any) string {
	// 如果 claims 为空，则返回空字符串
	if len(claims) == 0 {
		return ""
	}
	// 获取 org_id
	v, ok := claims["org_id"]
	// 如果 org_id 不存在，则返回空字符串
	if !ok || v == nil {
		// 如果 org_id 不存在，则返回空字符串
		return ""
	}
	// 获取 org_id 的值
	switch t := v.(type) {
	// 如果 org_id 的值为字符串，则返回字符串
	case string:
		// 如果 org_id 的值为字符串，则返回字符串
		return strings.TrimSpace(t)
	// 如果 org_id 的值为 fmt.Stringer，则返回字符串
	case fmt.Stringer:
		// 如果 org_id 的值为 fmt.Stringer，则返回字符串
		return strings.TrimSpace(t.String())
	default:
		// 如果 org_id 的值为其他类型，则返回字符串
		return strings.TrimSpace(fmt.Sprint(v))
	}
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
