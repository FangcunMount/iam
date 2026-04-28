package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	domainjwks "github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	domaintoken "github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestGeneratorAccessTokenUsesRegisteredAudienceAndParseRoundTrips(t *testing.T) {
	t.Parallel()

	generator, signingKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"qs-api", "collection-api"})
	principal := &authentication.Principal{
		AccountID: meta.MustFromUint64(1001),
		UserID:    meta.MustFromUint64(1002),
		TenantID:  meta.MustFromUint64(1),
		AMR:       []string{"pwd"},
		Claims: map[string]any{
			"display_name": "seed-user",
		},
	}

	token, err := generator.GenerateAccessToken(context.Background(), principal, 15*time.Minute)
	require.NoError(t, err)

	parsedJWT, rawClaims := parseRawClaims(t, token.Value, signingKey)
	require.Equal(t, "https://iam.fangcunmount.cn", parsedJWT.Issuer)
	require.Equal(t, []string{"qs-api", "collection-api"}, []string(parsedJWT.Audience))
	_, hasLegacyAudience := rawClaims["audience"]
	require.False(t, hasLegacyAudience)

	claims, err := generator.ParseAccessToken(context.Background(), token.Value)
	require.NoError(t, err)
	require.Equal(t, domaintoken.TokenTypeAccess, claims.TokenType)
	require.Equal(t, principal.UserID, claims.UserID)
	require.Equal(t, principal.AccountID, claims.AccountID)
	require.Equal(t, principal.TenantID, claims.TenantID)
	require.Equal(t, []string{"qs-api", "collection-api"}, claims.Audience)
	require.Equal(t, "https://iam.fangcunmount.cn", claims.Issuer)
	require.Equal(t, []string{"pwd"}, claims.AMR)
}

func TestGeneratorServiceTokenUsesRegisteredAudience(t *testing.T) {
	t.Parallel()

	generator, signingKey := newTestGenerator(t, "https://iam.fangcunmount.cn", []string{"ignored-default"})

	token, err := generator.GenerateServiceToken(
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
	manager := &jwksManagerStub{
		activeKey: newRSAJWKKey(t, kid, &privKey.PublicKey),
		keys: map[string]*domainjwks.Key{
			kid: newRSAJWKKey(t, kid, &privKey.PublicKey),
		},
	}
	resolver := &privateKeyResolverStub{
		keys: map[string]*rsa.PrivateKey{
			kid: privKey,
		},
	}

	return NewGenerator(issuer, accessAudience, manager, resolver), privKey
}

func newRSAJWKKey(t *testing.T, kid string, pubKey *rsa.PublicKey) *domainjwks.Key {
	t.Helper()

	n := base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes())
	return domainjwks.NewKey(kid, domainjwks.PublicJWK{
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		Kid: kid,
		N:   &n,
		E:   &e,
	})
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

type jwksManagerStub struct {
	activeKey *domainjwks.Key
	keys      map[string]*domainjwks.Key
}

func (s *jwksManagerStub) CreateKey(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*domainjwks.Key, error) {
	panic("unexpected call")
}

func (s *jwksManagerStub) GetActiveKey(ctx context.Context) (*domainjwks.Key, error) {
	return s.activeKey, nil
}

func (s *jwksManagerStub) GetKeyByKid(ctx context.Context, kid string) (*domainjwks.Key, error) {
	return s.keys[kid], nil
}

func (s *jwksManagerStub) RetireKey(ctx context.Context, kid string) error {
	panic("unexpected call")
}

func (s *jwksManagerStub) ForceRetireKey(ctx context.Context, kid string) error {
	panic("unexpected call")
}

func (s *jwksManagerStub) EnterGracePeriod(ctx context.Context, kid string) error {
	panic("unexpected call")
}

func (s *jwksManagerStub) CleanupExpiredKeys(ctx context.Context) (int, error) {
	panic("unexpected call")
}

func (s *jwksManagerStub) ListKeys(ctx context.Context, status domainjwks.KeyStatus, limit, offset int) ([]*domainjwks.Key, int64, error) {
	panic("unexpected call")
}

type privateKeyResolverStub struct {
	keys map[string]*rsa.PrivateKey
}

func (s *privateKeyResolverStub) ResolveSigningKey(ctx context.Context, kid, alg string) (any, error) {
	return s.keys[kid], nil
}
