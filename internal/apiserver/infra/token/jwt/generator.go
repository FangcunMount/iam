// Package jwt implements IAM access/service token encoding with JWS compact JWT.
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
	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type Generator struct {
	issuer              string
	accessTokenAudience []string
	keyMgmt             jwks.Manager
	privKeyResolver     jwks.PrivateKeyResolver
	claimsMapper        ClaimsMapper
}

var _ tokenapp.AccessTokenCodec = (*Generator)(nil)
var _ tokenapp.ClaimMapper = ClaimsMapper{}

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
		claimsMapper:        NewClaimsMapper(),
	}
}

type CustomClaims struct {
	TokenType  string            `json:"token_type,omitempty"`
	SessionID  string            `json:"sid,omitempty"`
	UserID     string            `json:"user_id,omitempty"`
	AccountID  string            `json:"account_id,omitempty"`
	TenantID   string            `json:"tenant_id,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	AMR        []string          `json:"amr,omitempty"`
	jwtv4.RegisteredClaims
}

func (g *Generator) ClaimMapper() tokenapp.ClaimMapper {
	return g.claimsMapper
}

func (g *Generator) IssueAccessToken(ctx context.Context, principal *authentication.Principal, expiresIn time.Duration) (*tokenapp.Token, error) {
	l := logger.L(ctx)
	l.Debugw("IssueAccessToken", "principal", fmt.Sprintf("%+v", principal), "expiresIn", expiresIn)
	now := time.Now()
	tokenID := uuid.NewString()

	claims := CustomClaims{
		TokenType:  string(tokenapp.TokenTypeAccess),
		SessionID:  principal.SessionID,
		UserID:     principal.UserID.String(),
		AccountID:  principal.AccountID.String(),
		TenantID:   principal.TenantID.String(),
		Attributes: cloneStringMap(g.claimsMapper.Encode(principal.Claims)),
		AMR:        cloneStrings(principal.AMR),
		RegisteredClaims: jwtv4.RegisteredClaims{
			ID:        tokenID,
			Subject:   principal.UserID.String(),
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

	return tokenapp.NewAccessToken(
		tokenID,
		tokenString,
		principal.SessionID,
		principal.UserID,
		principal.AccountID,
		principal.TenantID,
		expiresIn,
	), nil
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

		key, err := g.keyMgmt.GetKeyByKid(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get key %s: %w", kid, err)
		}
		if key == nil {
			return nil, fmt.Errorf("key not found for kid %s", kid)
		}
		if key.JWK.Kty != "RSA" {
			return nil, fmt.Errorf("unsupported key kty for verification: %s", key.JWK.Kty)
		}
		if key.JWK.N == nil || key.JWK.E == nil {
			return nil, fmt.Errorf("missing RSA parameters in JWK for kid %s", kid)
		}

		nBytes, err := base64.RawURLEncoding.DecodeString(*key.JWK.N)
		if err != nil {
			return nil, fmt.Errorf("failed to base64url-decode n for kid %s: %w", kid, err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(*key.JWK.E)
		if err != nil {
			return nil, fmt.Errorf("failed to base64url-decode e for kid %s: %w", kid, err)
		}
		n := new(big.Int).SetBytes(nBytes)
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}
		if e == 0 {
			return nil, fmt.Errorf("invalid exponent parsed for kid %s", kid)
		}
		return &rsa.PublicKey{N: n, E: e}, nil
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
	return tokenapp.NewTokenClaims(
		tokenType,
		claims.ID,
		claims.Subject,
		claims.SessionID,
		parseStringID(claims.UserID),
		parseStringID(claims.AccountID),
		parseStringID(claims.TenantID),
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
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodRS256, claims)
	token.Header["typ"] = headerTypeJWT
	token.Header["kid"] = activeKey.Kid
	return token.SignedString(rsaPrivKey)
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
