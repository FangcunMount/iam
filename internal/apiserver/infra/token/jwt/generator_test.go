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

	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestGeneratorAccessTokenUsesRegisteredAudienceAndParseRoundTrips(t *testing.T) {
	t.Parallel()

	generator, signingKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api", "collection-api"})
	principal := &tokenapp.Principal{
		AccountID: meta.MustFromUint64(1001),
		UserID:    meta.MustFromUint64(1002),
		TenantID:  meta.MustFromUint64(1),
		AMR:       []string{"pwd"},
		Claims: map[string]any{
			"display_name": "seed-user",
			"kid":          "must-not-enter-payload",
		},
	}

	token, err := generator.IssueAccessToken(context.Background(), principal, 15*time.Minute)
	require.NoError(t, err)

	parsedJWT, rawClaims := parseRawClaims(t, token.Value, signingKey)
	require.Equal(t, "https://iam.fangcunmount.cn", parsedJWT.Issuer)
	require.Equal(t, []string{"qs-api", "collection-api"}, []string(parsedJWT.Audience))
	_, hasLegacyAudience := rawClaims["audience"]
	require.False(t, hasLegacyAudience)

	claims, err := generator.VerifyAccessToken(context.Background(), token.Value)
	require.NoError(t, err)
	require.Equal(t, tokenapp.TokenTypeAccess, claims.TokenType)
	require.Equal(t, principal.UserID, claims.UserID)
	require.Equal(t, principal.AccountID, claims.AccountID)
	require.Equal(t, principal.TenantID, claims.TenantID)
	require.Equal(t, []string{"qs-api", "collection-api"}, claims.Audience)
	require.Equal(t, "https://iam.fangcunmount.cn", claims.Issuer)
	require.Equal(t, []string{"pwd"}, claims.AMR)
}

func TestGeneratorTokenUsesJWSCompactHeaderPayloadSignatureContract(t *testing.T) {
	t.Parallel()

	generator, _ := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	token, err := generator.IssueAccessToken(context.Background(), &tokenapp.Principal{
		AccountID: meta.MustFromUint64(1001),
		UserID:    meta.MustFromUint64(1002),
		TenantID:  meta.MustFromUint64(1),
		Claims: map[string]any{
			"kid": "payload-kid-is-reserved",
			"alg": "payload-alg-is-reserved",
			"typ": "payload-typ-is-reserved",
		},
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

func TestGeneratorRejectsNoneAlgorithm(t *testing.T) {
	t.Parallel()

	generator, _ := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api"})
	claims := CustomClaims{
		TokenType: string(tokenapp.TokenTypeAccess),
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

func parseRawClaims(t *testing.T, tokenValue string, key *rsa.PrivateKey) (*CustomClaims, jwtv4.MapClaims) {
	t.Helper()

	var claims CustomClaims
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
	privateKey *rsa.PrivateKey
	publicKeys map[string]*rsa.PublicKey
}

func (s *signingKeySourceStub) ActiveSigningKey(context.Context) (string, *rsa.PrivateKey, error) {
	return s.kid, s.privateKey, nil
}

func (s *signingKeySourceStub) VerificationKey(_ context.Context, kid string) (*rsa.PublicKey, error) {
	return s.publicKeys[kid], nil
}
