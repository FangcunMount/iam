package session

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/proof"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type sessionTokenCapabilitiesStub struct {
	captured *authentication.Principal
}

func (s *sessionTokenCapabilitiesStub) IssueAuthentication(ctx context.Context, principal *authentication.Principal) (*tokenapp.TokenPair, error) {
	s.captured = principal
	access := tokenapp.NewAccessToken(
		"access-id",
		"access-value",
		"session-id",
		principal.UserID,
		principal.LoginIdentityID,
		meta.ZeroID,
		time.Minute,
	)
	refresh := tokenapp.NewRefreshToken(
		"refresh-id",
		"refresh-value",
		"session-id",
		principal.UserID,
		principal.LoginIdentityID,
		meta.ZeroID,
		nil,
		nil,
		time.Hour,
	)
	return tokenapp.NewTokenPair(access, refresh), nil
}

func (s *sessionTokenCapabilitiesStub) RefreshToken(ctx context.Context, refreshToken string) (*tokenapp.TokenRefreshResult, error) {
	return nil, nil
}

func (s *sessionTokenCapabilitiesStub) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	return nil
}

func (s *sessionTokenCapabilitiesStub) RevokeRefreshToken(ctx context.Context, tokenValue string) error {
	return nil
}

type sessionTokenCapabilities interface {
	tokenapp.AuthenticationGrantIssuer
	tokenapp.Refresher
	tokenapp.Revoker
}

func newSessionServiceForTest(t *testing.T, tokens sessionTokenCapabilities, auth *authentication.Authenticator) ApplicationService {
	t.Helper()

	signIn := signin.New(signin.Dependencies{
		AuthenticationGrantIssuer: tokens,
		Authenticator:             auth,
		MethodRegistry:            method.DefaultSelector(),
		ProofFactory:              proof.DefaultFactory(nil, nil),
		CredentialRecorder:        nil,
	})
	svc, err := NewApplicationService(Dependencies{
		Refresher: tokens,
		Revoker:   tokens,
		SignIn:    signIn,
	})
	require.NoError(t, err)
	return svc
}

func TestMethodSelectorUsesAuthMethodAsAuthority(t *testing.T) {
	t.Parallel()

	selector := method.DefaultSelector()
	selected, err := selector.Select(context.Background(), LoginRequest{
		AuthMethod: AuthMethodPassword,
		Payload: PasswordPayload{
			Username: "alice",
			Password: "secret",
		},
	})

	require.NoError(t, err)
	require.Equal(t, method.CredentialKindPassword, selected.CredentialKind)
	payload, ok := selected.Payload.(PasswordPayload)
	require.True(t, ok)
	require.Equal(t, "alice", payload.Username)
	require.Equal(t, "secret", payload.Password)
}

func TestLoginPayloadFailureUsesPayloadInvalidCode(t *testing.T) {
	t.Parallel()

	auth := authentication.NewAuthenticator()
	tokens := &sessionTokenCapabilitiesStub{}
	svc := newSessionServiceForTest(t, tokens, auth)

	result, err := svc.Login(context.Background(), LoginRequest{
		AuthMethod: AuthMethodPassword,
		Payload: PhoneOTPPayload{
			PhoneE164: "+8613800138000",
			OTP:       "123456",
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, code.ErrPayloadInvalid, perrors.ParseCoder(err).Code())
	require.Nil(t, tokens.captured)
}

func TestLoginProofFailureUsesProofBuildFailedCode(t *testing.T) {
	t.Parallel()

	auth := authentication.NewAuthenticator()
	tokens := &sessionTokenCapabilitiesStub{}
	svc := newSessionServiceForTest(t, tokens, auth)

	result, err := svc.Login(context.Background(), LoginRequest{
		AuthMethod: AuthMethodWecom,
		Payload: WecomPayload{
			CorpID: "corp-id",
			Code:   "auth-code",
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, code.ErrProofBuildFailed, perrors.ParseCoder(err).Code())
	require.Nil(t, tokens.captured)
}
