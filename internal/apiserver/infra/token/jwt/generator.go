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
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type SigningKeySource interface {
	ActiveSigningKey(ctx context.Context) (kid string, privateKey *rsa.PrivateKey, err error)
	VerificationKey(ctx context.Context, kid string) (*rsa.PublicKey, error)
}

type Generator struct {
	issuer              string
	accessTokenAudience []string
	keySource           SigningKeySource
	attributeEncoder jwtAttributeEncoder
}

var _ tokenapp.AccessTokenCodec = (*Generator)(nil)

func NewGenerator(
	issuer string,
	accessTokenAudience []string,
	keySource SigningKeySource,
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
		keySource:           keySource,
		attributeEncoder: newJWTAttributeEncoder(),
	}
}

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

func (g *Generator) IssueAccessToken(ctx context.Context, subject *tokenapp.AccessTokenSubject, expiresIn time.Duration) (*tokenapp.Token, error) {
	l := logger.L(ctx)
	l.Debugw("IssueAccessToken", "subject", fmt.Sprintf("%+v", subject), "expiresIn", expiresIn)
	now := time.Now()
	tokenID := uuid.NewString()
	loginIdentityID := subject.LoginIdentityID
	authMethod := subject.AuthMethod
	realm := subject.Realm

	claims := CustomClaims{
		TokenType:       string(tokenapp.TokenTypeAccess),
		SessionID:       subject.SessionID,
		UserID:          subject.UserID.String(),
		LoginIdentityID: loginIdentityID.String(),
		OrgID:           businessOrgIDClaim(subject.Claims),
		TenantID:        tokenapp.TenantDomainFromClaims(subject.Claims, subject.Realm),
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

	tokenString, err := g.signClaims(ctx, claims)
	if err != nil {
		return nil, err
	}

	token := tokenapp.NewAccessToken(
		tokenID,
		tokenString,
		subject.SessionID,
		subject.UserID,
		loginIdentityID,
		subject.TenantID,
		expiresIn,
	)
	token.LoginIdentityID = loginIdentityID
	token.AuthMethod = authMethod
	token.Realm = realm
	return token, nil
}

func (g *Generator) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*tokenapp.Token, error) {
	now := time.Now()
	tokenID := uuid.NewString()
	claims := CustomClaims{
		TokenType:  string(tokenapp.TokenTypeService),
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
	tokenString, err := g.signClaims(ctx, claims)
	if err != nil {
		return nil, err
	}
	return tokenapp.NewServiceToken(tokenID, tokenString, subject, audience, attributes, expiresIn), nil
}

func (g *Generator) VerifyAccessToken(ctx context.Context, tokenValue string) (*tokenapp.TokenClaims, error) {
	parsed, err := jwtv4.ParseWithClaims(tokenValue, &CustomClaims{}, func(token *jwtv4.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtv4.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kidInterface, ok := token.Header["kid"]
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}
		kid, ok := kidInterface.(string)
		if !ok {
			return nil, fmt.Errorf("invalid kid type in token header")
		}

		key, err := g.keySource.VerificationKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get key %s: %w", kid, err)
		}
		if key == nil {
			return nil, fmt.Errorf("key not found for kid %s", kid)
		}
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := parsed.Claims.(*CustomClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	tokenType := tokenapp.TokenType(claims.TokenType)
	if tokenType == "" {
		tokenType = tokenapp.TokenTypeAccess
	}
	loginIdentityID := parseStringID(claims.LoginIdentityID)
	orgID := parseStringID(claims.OrgID)
	tenantDomain, _ := parseTenantIDClaim(claims.TenantID)
	tokenClaims := tokenapp.NewTokenClaims(
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
	tokenClaims.AuthMethod = claims.AuthMethod
	tokenClaims.Realm = claims.Realm
	return tokenClaims, nil
}

func (g *Generator) signClaims(ctx context.Context, claims CustomClaims) (string, error) {
	kid, rsaPrivKey, err := g.keySource.ActiveSigningKey(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to resolve active signing key: %w", err)
	}
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodRS256, claims)
	token.Header["typ"] = headerTypeJWT
	token.Header["kid"] = kid
	return token.SignedString(rsaPrivKey)
}

// businessOrgIDClaim 从 Principal.Claims 读取业务侧提供的 org_id，IAM 不生成默认值。
func businessOrgIDClaim(claims map[string]any) string {
	if len(claims) == 0 {
		return ""
	}
	v, ok := claims["org_id"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
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

func numericDateTime(v *jwtv4.NumericDate) time.Time {
	if v == nil {
		return time.Time{}
	}
	return v.Time
}
