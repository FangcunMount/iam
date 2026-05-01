package token

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type verifyerStub struct {
	claims *TokenClaims
	err    error
}

func (s *verifyerStub) VerifyAccessToken(context.Context, string) (*TokenClaims, error) {
	return s.claims, s.err
}

type serviceTokenIssuerStub struct {
	subject    string
	audience   []string
	attributes map[string]string
	ttl        time.Duration
	pair       *TokenPair
}

func (s *serviceTokenIssuerStub) IssueServiceToken(_ context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	s.subject = subject
	s.audience = audience
	s.attributes = attributes
	s.ttl = ttl
	return s.pair, nil
}

func TestTokenApplicationServiceVerifyTokenHonorsExpectedIssuerAndAudience(t *testing.T) {
	svc := &tokenApplicationService{
		tokenVerifier: &verifyerStub{
			claims: NewTokenClaims(
				TokenTypeAccess,
				"tid",
				"user:1",
				"sid-1",
				meta.FromUint64(1),
				meta.FromUint64(2),
				meta.FromUint64(3),
				"https://iam.fangcunmount.cn",
				[]string{"qs-api", "collection-api"},
				nil,
				[]string{"pwd"},
				time.Now(),
				time.Now().Add(time.Minute),
			),
		},
	}

	okResult, err := svc.VerifyToken(context.Background(), VerifyTokenRequest{
		AccessToken:      "token",
		ExpectedIssuer:   "https://iam.fangcunmount.cn",
		ExpectedAudience: []string{"qs-api"},
	})
	require.NoError(t, err)
	require.True(t, okResult.Valid)
	require.NotNil(t, okResult.Claims)

	issuerMismatch, err := svc.VerifyToken(context.Background(), VerifyTokenRequest{
		AccessToken:    "token",
		ExpectedIssuer: "https://issuer.invalid",
	})
	require.NoError(t, err)
	require.False(t, issuerMismatch.Valid)
	require.Nil(t, issuerMismatch.Claims)

	audienceMismatch, err := svc.VerifyToken(context.Background(), VerifyTokenRequest{
		AccessToken:      "token",
		ExpectedAudience: []string{"wrong-audience"},
	})
	require.NoError(t, err)
	require.False(t, audienceMismatch.Valid)
	require.Nil(t, audienceMismatch.Claims)
}

func TestTokenApplicationServiceIssueServiceTokenUsesServiceIssuer(t *testing.T) {
	t.Parallel()

	serviceToken := NewServiceToken(
		"service-token-id",
		"service-token-value",
		"service:mailer",
		[]string{"iam-api"},
		map[string]string{"scope": "send"},
		time.Minute,
	)
	issuer := &serviceTokenIssuerStub{pair: NewTokenPair(serviceToken, nil)}
	svc := &tokenApplicationService{serviceTokenIssuer: issuer}

	result, err := svc.IssueServiceToken(context.Background(), IssueServiceTokenRequest{
		Subject:    "service:mailer",
		Audience:   []string{"iam-api"},
		Attributes: map[string]string{"scope": "send"},
		TTL:        time.Minute,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.TokenPair.AccessToken)
	require.Nil(t, result.TokenPair.RefreshToken)
	require.Equal(t, "service:mailer", issuer.subject)
	require.Equal(t, []string{"iam-api"}, issuer.audience)
	require.Equal(t, map[string]string{"scope": "send"}, issuer.attributes)
	require.Equal(t, time.Minute, issuer.ttl)
}
