package reauth

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type tokenVerifierStub struct {
	claims *tokenapp.TokenClaims
	token  string
	err    error
}

func (s *tokenVerifierStub) VerifyAccessToken(_ context.Context, tokenValue string) (*tokenapp.TokenClaims, error) {
	s.token = tokenValue
	if s.err != nil {
		return nil, s.err
	}
	return s.claims, nil
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

	verifier := &tokenVerifierStub{err: perrors.WithCode(code.ErrExpired, "expired")}
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
