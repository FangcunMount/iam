package token

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type verifyerStub struct {
	claims *TokenClaims
	err    error
}

func (s *verifyerStub) VerifyAccessToken(context.Context, string) (*TokenClaims, error) {
	return s.claims, s.err
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
