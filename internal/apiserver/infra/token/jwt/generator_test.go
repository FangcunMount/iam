package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestGeneratorAccessTokenUsesRegisteredAudienceAndParseRoundTrips(t *testing.T) {
	t.Parallel()

	generator, signingKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api", "collection-api"})
	subject := &tokendomain.AccessTokenSubject{
		LoginIdentityID: meta.MustFromUint64(1001),
		UserID:          meta.MustFromUint64(1002),
		SessionID:       "sid-1002",
		TenantDomain:    "fangcun",
		OrgID:           "1",
		AMR:             []string{"pwd"},
		Attributes:      map[string]string{"display_name": "seed-user"},
	}

	token, err := generator.IssueAccessToken(context.Background(), subject, 15*time.Minute)
	require.NoError(t, err)

	parsedJWT, rawClaims := parseRawClaims(t, token.Value, signingKey)
	require.Equal(t, "https://iam.fangcunmount.cn", parsedJWT.Issuer)
	require.Equal(t, []string{"qs-api", "collection-api"}, []string(parsedJWT.Audience))
	_, hasLegacyAudience := rawClaims["audience"]
	require.False(t, hasLegacyAudience)

	claims, err := generator.VerifyAccessToken(context.Background(), token.Value)
	require.NoError(t, err)
	require.Equal(t, tokendomain.TokenTypeAccess, claims.TokenType)
	require.Equal(t, subject.UserID, claims.UserID)
	require.Equal(t, subject.LoginIdentityID, claims.LoginIdentityID)
	require.Equal(t, meta.MustFromUint64(1), claims.OrgID)
	require.Equal(t, "fangcun", claims.TenantDomain)
	require.Equal(t, []string{"qs-api", "collection-api"}, claims.Audience)
	require.Equal(t, "https://iam.fangcunmount.cn", claims.Issuer)
	require.Equal(t, []string{"pwd"}, claims.AMR)
}

func TestGeneratorTokenUsesJWSCompactHeaderPayloadSignatureContract(t *testing.T) {
	t.Parallel()

	generator, _ := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	token, err := generator.IssueAccessToken(context.Background(), &tokendomain.AccessTokenSubject{
		LoginIdentityID: meta.MustFromUint64(1001),
		UserID:          meta.MustFromUint64(1002),
		SessionID:       "sid-1002",
		TenantDomain:    "fangcun",
		Attributes:      map[string]string{"display_name": "seed-user"},
	}, time.Minute)
	require.NoError(t, err)

	parts := strings.Split(token.Value, ".")
	require.Len(t, parts, 3)
	for _, part := range parts {
		require.NotContains(t, part, "=")
		_, err := base64.RawURLEncoding.DecodeString(part)
		require.NoError(t, err)
	}

	header := decodeJWTPart[map[string]any](t, parts[0])
	require.Equal(t, "JWT", header["typ"])
	require.Equal(t, "RS256", header["alg"])
	require.Equal(t, "test-key", header["kid"])

	payload := decodeJWTPart[map[string]any](t, parts[1])
	require.Equal(t, "https://iam.fangcunmount.cn", payload["iss"])
	require.Equal(t, "1002", payload["sub"])
	require.NotContains(t, payload, "kid")
	require.NotContains(t, payload, "alg")
	require.NotContains(t, payload, "typ")
	require.Contains(t, payload, "jti")
	require.Contains(t, payload, "exp")
	require.Contains(t, payload, "iat")
	require.Contains(t, payload, "nbf")
}

func TestGeneratorLegacyNumericTenantIDDoesNotInferOrg(t *testing.T) {
	t.Parallel()

	generator, _ := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	token, err := generator.IssueAccessToken(context.Background(), &tokendomain.AccessTokenSubject{
		UserID:          meta.MustFromUint64(1002),
		LoginIdentityID: meta.MustFromUint64(1001),
		SessionID:       "sid-1002",
		TenantDomain:    "fangcun",
	}, time.Minute)
	require.NoError(t, err)

	// 模拟历史 token：tenant_id 为数值、无 org_id。
	legacy := strings.Replace(token.Value, `"tenant_id":"fangcun"`, `"tenant_id":"1"`, 1)

	claims, err := generator.VerifyAccessToken(context.Background(), legacy)
	require.NoError(t, err)
	require.Equal(t, tenant.DefaultID, claims.TenantDomain)
	require.True(t, claims.OrgID.IsZero())
}

func TestGeneratorRejectsNoneAlgorithm(t *testing.T) {
	t.Parallel()

	generator, _ := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	claims := jwtPayloadClaims{
		TokenType: string(tokendomain.TokenTypeAccess),
		UserID:    "1002",
		RegisteredClaims: jwtv4.RegisteredClaims{
			ID:        "none-token",
			Subject:   "1002",
			Issuer:    "https://iam.fangcunmount.cn",
			Audience:  jwtv4.ClaimStrings{"qs-api"},
			IssuedAt:  jwtv4.NewNumericDate(time.Now()),
			ExpiresAt: jwtv4.NewNumericDate(time.Now().Add(time.Minute)),
			NotBefore: jwtv4.NewNumericDate(time.Now()),
		},
	}
	raw, err := jwtv4.NewWithClaims(jwtv4.SigningMethodNone, claims).SignedString(jwtv4.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = generator.VerifyAccessToken(context.Background(), raw)
	require.Error(t, err)
}

func TestGeneratorRejectsAlgorithmsAndKeyMetadataOutsideProfile(t *testing.T) {
	t.Parallel()

	generator, privateKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	claims := jwtPayloadClaims{RegisteredClaims: jwtv4.RegisteredClaims{
		Subject:   "1002",
		ExpiresAt: jwtv4.NewNumericDate(time.Now().Add(time.Minute)),
	}}

	tests := []struct {
		name   string
		method jwtv4.SigningMethod
		key    any
		kid    string
	}{
		{name: "HS256", method: jwtv4.SigningMethodHS256, key: []byte("secret"), kid: "test-key"},
		{name: "RS384", method: jwtv4.SigningMethodRS384, key: privateKey, kid: "test-key"},
		{name: "RS512", method: jwtv4.SigningMethodRS512, key: privateKey, kid: "test-key"},
		{name: "missing kid", method: jwtv4.SigningMethodRS256, key: privateKey},
		{name: "unknown kid", method: jwtv4.SigningMethodRS256, key: privateKey, kid: "unknown-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := jwtv4.NewWithClaims(tt.method, claims)
			if tt.kid != "" {
				token.Header["kid"] = tt.kid
			}
			raw, err := token.SignedString(tt.key)
			require.NoError(t, err)

			_, err = generator.VerifyAccessToken(context.Background(), raw)
			require.Error(t, err)
		})
	}
}

func TestGeneratorRejectsJWKAlgorithmMismatch(t *testing.T) {
	t.Parallel()

	generator, privateKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	generator.keySource.(*signingKeySourceStub).keyAlgs = map[string]string{"test-key": "RS384"}
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodRS256, jwtPayloadClaims{
		RegisteredClaims: jwtv4.RegisteredClaims{ExpiresAt: jwtv4.NewNumericDate(time.Now().Add(time.Minute))},
	})
	token.Header["kid"] = "test-key"
	raw, err := token.SignedString(privateKey)
	require.NoError(t, err)

	_, err = generator.VerifyAccessToken(context.Background(), raw)
	require.Error(t, err)
}

func TestGeneratorFailsClosedWhenActiveKeyAlgorithmIsNotRS256(t *testing.T) {
	t.Parallel()

	generator, _ := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	generator.keySource.(*signingKeySourceStub).algorithm = "RS384"

	token, err := generator.IssueServiceToken(context.Background(), "service", []string{"api"}, nil, time.Minute)
	require.Error(t, err)
	require.Nil(t, token)
}

func TestGeneratorOmitsSensitiveAttributesAndAuthMethodRealm(t *testing.T) {
	t.Parallel()

	generator, signingKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	token, err := generator.IssueAccessToken(context.Background(), &tokendomain.AccessTokenSubject{
		UserID:          meta.MustFromUint64(1002),
		LoginIdentityID: meta.MustFromUint64(1001),
		SessionID:       "sid-1002",
		TenantDomain:    "fangcun",
		AuthenticatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Attributes:      map[string]string{"auth_time": "2026-01-02T03:04:05Z"},
	}, time.Minute)
	require.NoError(t, err)

	_, rawClaims := parseRawClaims(t, token.Value, signingKey)
	require.NotContains(t, rawClaims, "auth_method")
	require.NotContains(t, rawClaims, "realm")
	require.Equal(t, float64(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Unix()), rawClaims["auth_time"])
	attrs, _ := rawClaims["attributes"].(map[string]any)
	require.NotContains(t, attrs, "phone_number")
	require.NotContains(t, attrs, "wx_openid")
	require.Equal(t, "2026-01-02T03:04:05Z", attrs["auth_time"])
}

func TestGeneratorRejectsIssuerMismatch(t *testing.T) {
	t.Parallel()

	generator, signingKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	claims := jwtPayloadClaims{
		TokenType: string(tokendomain.TokenTypeAccess),
		UserID:    "1002",
		RegisteredClaims: jwtv4.RegisteredClaims{
			ID:        "issuer-mismatch",
			Subject:   "1002",
			Issuer:    "https://issuer.invalid",
			Audience:  jwtv4.ClaimStrings{"qs-api"},
			IssuedAt:  jwtv4.NewNumericDate(time.Now()),
			ExpiresAt: jwtv4.NewNumericDate(time.Now().Add(time.Minute)),
			NotBefore: jwtv4.NewNumericDate(time.Now()),
		},
	}
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodRS256, claims)
	token.Header["typ"] = "JWT"
	token.Header["kid"] = "test-key"
	raw, err := token.SignedString(signingKey)
	require.NoError(t, err)

	_, err = generator.VerifyAccessToken(context.Background(), raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected token issuer")
}

func TestGeneratorServiceTokenUsesRegisteredAudience(t *testing.T) {
	t.Parallel()

	generator, signingKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"ignored-default"})

	token, err := generator.IssueServiceToken(
		context.Background(),
		"svc:report-worker",
		[]string{"collection-api"},
		map[string]string{"scope": "internal"},
		10*time.Minute,
	)
	require.NoError(t, err)

	parsedJWT, rawClaims := parseRawClaims(t, token.Value, signingKey)
	require.Equal(t, "https://iam.fangcunmount.cn", parsedJWT.Issuer)
	require.Equal(t, []string{"collection-api"}, []string(parsedJWT.Audience))
	_, hasLegacyAudience := rawClaims["audience"]
	require.False(t, hasLegacyAudience)
}

func newTestGenerator(t *testing.T, issuer string, accessAudience []string) (*Generator, *rsa.PrivateKey) {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	kid := "test-key"
	keySource := &signingKeySourceStub{
		kid:        kid,
		privateKey: privKey,
		publicKeys: map[string]*rsa.PublicKey{
			kid: &privKey.PublicKey,
		},
	}

	return NewGenerator(issuer, accessAudience, keySource), privKey
}

func parseRawClaims(t *testing.T, tokenValue string, key *rsa.PrivateKey) (*jwtPayloadClaims, jwtv4.MapClaims) {
	t.Helper()

	var claims jwtPayloadClaims
	parsed, err := jwtv4.ParseWithClaims(tokenValue, &claims, func(token *jwtv4.Token) (any, error) {
		return &key.PublicKey, nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	parser := jwtv4.Parser{}
	rawClaims := jwtv4.MapClaims{}
	_, _, err = parser.ParseUnverified(tokenValue, rawClaims)
	require.NoError(t, err)

	return &claims, rawClaims
}

func decodeJWTPart[T any](t *testing.T, raw string) T {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	require.NoError(t, err)
	var out T
	require.NoError(t, json.Unmarshal(decoded, &out))
	return out
}

type signingKeySourceStub struct {
	kid        string
	algorithm  string
	privateKey *rsa.PrivateKey
	publicKeys map[string]*rsa.PublicKey
	keyAlgs    map[string]string
}

func (s *signingKeySourceStub) ActiveSigningKey(context.Context) (*SigningKey, error) {
	algorithm := s.algorithm
	if algorithm == "" {
		algorithm = "RS256"
	}
	return &SigningKey{Kid: s.kid, Algorithm: algorithm, PrivateKey: s.privateKey}, nil
}

func (s *signingKeySourceStub) VerificationKey(_ context.Context, kid string) (*VerificationKey, error) {
	publicKey := s.publicKeys[kid]
	if publicKey == nil {
		return nil, nil
	}
	algorithm := "RS256"
	if s.keyAlgs != nil && s.keyAlgs[kid] != "" {
		algorithm = s.keyAlgs[kid]
	}
	return &VerificationKey{Kid: kid, Algorithm: algorithm, PublicKey: publicKey}, nil
}
