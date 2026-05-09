package reauth

import (
	"context"
	"errors"
	"testing"

	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type tokenVerifierStub struct {
	claims *tokenapp.TokenClaims
	token  string
	err    error
	code   int
}

func (s *tokenVerifierStub) VerifyToken(_ context.Context, req tokenapp.VerifyTokenRequest) (*tokenapp.TokenVerifyResult, error) {
	s.token = req.AccessToken
	if s.err != nil {
		return nil, s.err
	}
	if s.code != 0 {
		return &tokenapp.TokenVerifyResult{Valid: false, FailureCode: s.code}, nil
	}
	return &tokenapp.TokenVerifyResult{Valid: s.claims != nil, Claims: s.claims}, nil
}

func TestTokenReAuthenticatorMapsClaimsToPrincipal(t *testing.T) {
	t.Parallel()

	verifier := &tokenVerifierStub{claims: &tokenapp.TokenClaims{
		UserID:     meta.FromUint64(1001),
		AccountID:  meta.FromUint64(2002),
		TenantID:   meta.FromUint64(3003),
		SessionID:  "session-id",
		AMR:        []string{"pwd"},
		Attributes: map[string]string{"scope": "profile"},
	}}
	authenticator := NewTokenReAuthenticator(verifier)

	decision, err := authenticator.Reauthenticate(context.Background(), "access-token")

	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, "access-token", verifier.token)
	require.Equal(t, meta.FromUint64(1001), decision.Principal.UserID)
	require.Equal(t, meta.FromUint64(2002), decision.Principal.AccountID)
	require.Equal(t, meta.FromUint64(3003), decision.Principal.TenantID)
	require.Equal(t, "session-id", decision.Principal.SessionID)
	require.Equal(t, []string{"pwd"}, decision.Principal.AMR)
	require.Equal(t, "profile", decision.Principal.Claims["scope"])
}

func TestTokenReAuthenticatorMapsTokenFailureToDecision(t *testing.T) {
	t.Parallel()

	verifier := &tokenVerifierStub{code: code.ErrExpired}
	authenticator := NewTokenReAuthenticator(verifier)

	decision, err := authenticator.Reauthenticate(context.Background(), "expired-token")

	require.NoError(t, err)
	require.False(t, decision.OK)
	require.Equal(t, code.ErrExpired, decision.Code)
}

func TestTokenReAuthenticatorReturnsSystemError(t *testing.T) {
	t.Parallel()

	boom := errors.New("token store unavailable")
	verifier := &tokenVerifierStub{err: boom}
	authenticator := NewTokenReAuthenticator(verifier)

	decision, err := authenticator.Reauthenticate(context.Background(), "access-token")

	require.ErrorIs(t, err, boom)
	require.False(t, decision.OK)
}
