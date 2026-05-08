package login

import (
	"context"
	"errors"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/proof"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/reauth"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
	"github.com/stretchr/testify/require"
)

type loginTokenIssuerStub struct {
	captured *authentication.Principal
}

func (s *loginTokenIssuerStub) IssueToken(ctx context.Context, principal *authentication.Principal) (*tokenapp.TokenPair, error) {
	s.captured = principal
	access := tokenapp.NewAccessToken(
		"access-id",
		"access-value",
		"session-id",
		principal.UserID,
		principal.AccountID,
		principal.TenantID,
		time.Minute,
	)
	refresh := tokenapp.NewRefreshToken(
		"refresh-id",
		"refresh-value",
		"session-id",
		principal.UserID,
		principal.AccountID,
		principal.TenantID,
		nil,
		nil,
		time.Hour,
	)
	return tokenapp.NewTokenPair(access, refresh), nil
}

func (s *loginTokenIssuerStub) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*tokenapp.TokenPair, error) {
	return nil, nil
}

func (s *loginTokenIssuerStub) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	return nil
}

type loginTokenRevokerStub struct{}

func (s loginTokenRevokerStub) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	return nil
}

func (s loginTokenRevokerStub) RevokeRefreshToken(ctx context.Context, tokenValue string) error {
	return nil
}

type loginAccountRepoStub struct {
	enabled bool
	locked  bool
}

func (s *loginAccountRepoStub) FindAccountByUsername(ctx context.Context, tenantID meta.ID, username string) (*authentication.UsernameLoginLookup, error) {
	return nil, nil
}

func (s *loginAccountRepoStub) GetAccountStatus(ctx context.Context, accountID meta.ID) (bool, bool, error) {
	return s.enabled, s.locked, nil
}

type loginTokenVerifierStub struct {
	userID    meta.ID
	accountID meta.ID
	tenantID  meta.ID
	sessionID string
	amr       []string
	attrs     map[string]string
	token     string
	err       error
}

func (s *loginTokenVerifierStub) VerifyAccessToken(ctx context.Context, tokenValue string) (*tokenapp.TokenClaims, error) {
	s.token = tokenValue
	if s.err != nil {
		return nil, s.err
	}
	return &tokenapp.TokenClaims{
		UserID:     s.userID,
		AccountID:  s.accountID,
		TenantID:   s.tenantID,
		SessionID:  s.sessionID,
		AMR:        s.amr,
		Attributes: s.attrs,
	}, nil
}

func newLoginServiceForTest(t *testing.T, issuer tokenapp.Issuer, auth *authentication.Authenticator, reauth ReAuthenticator) LoginApplicationService {
	t.Helper()

	svc, err := NewLoginApplicationService(Dependencies{
		TokenIssuer:     issuer,
		TokenRevoker:    loginTokenRevokerStub{},
		Authenticator:   auth,
		MethodRegistry:  method.DefaultSelector(),
		ProofFactory:    proof.DefaultFactory(nil, nil, WecomConfig{}),
		ReAuthenticator: reauth,
	})
	require.NoError(t, err)
	return svc
}

func TestReauthenticateDefaultsMissingTenantID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tokenTenant meta.ID
		wantTenant  uint64
	}{
		{
			name:        "fills default tenant for zero tenant",
			tokenTenant: meta.FromUint64(0),
			wantTenant:  tenant.DefaultTenantID,
		},
		{
			name:        "keeps explicit tenant",
			tokenTenant: meta.FromUint64(77),
			wantTenant:  77,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			auth := authentication.NewAuthenticator()

			issuer := &loginTokenIssuerStub{}
			verifier := &loginTokenVerifierStub{
				userID:    meta.FromUint64(1001),
				accountID: meta.FromUint64(2002),
				tenantID:  tc.tokenTenant,
				sessionID: "session-id",
				amr:       []string{"pwd"},
				attrs:     map[string]string{"scope": "profile"},
			}
			svc := newLoginServiceForTest(t, issuer, auth, reauth.NewTokenReAuthenticator(verifier))

			result, err := svc.Reauthenticate(context.Background(), " access-token-value ")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Principal)

			require.Equal(t, tc.wantTenant, result.TenantID.Uint64())
			require.Equal(t, tc.wantTenant, result.Principal.TenantID.Uint64())
			require.Equal(t, meta.FromUint64(1001), result.UserID)
			require.Equal(t, meta.FromUint64(2002), result.AccountID)
			require.Equal(t, "session-id", result.Principal.SessionID)
			require.Equal(t, []string{"pwd"}, result.Principal.AMR)
			require.Equal(t, "profile", result.Principal.Claims["scope"])
			require.Equal(t, "access-token-value", verifier.token)
			require.Nil(t, issuer.captured)
		})
	}
}

func TestReauthenticateTokenVerifierFailureMapsToAuthFailure(t *testing.T) {
	t.Parallel()

	auth := authentication.NewAuthenticator()
	issuer := &loginTokenIssuerStub{}
	verifier := &loginTokenVerifierStub{err: perrors.WithCode(code.ErrExpired, "expired")}
	svc := newLoginServiceForTest(t, issuer, auth, reauth.NewTokenReAuthenticator(verifier))

	result, err := svc.Reauthenticate(context.Background(), "expired-token")

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, code.ErrExpired, perrors.ParseCoder(err).Code())
	require.Nil(t, issuer.captured)
}

func TestReauthenticateTokenVerifierSystemErrorReturnsError(t *testing.T) {
	t.Parallel()

	boom := errors.New("token store unavailable")
	auth := authentication.NewAuthenticator()
	issuer := &loginTokenIssuerStub{}
	verifier := &loginTokenVerifierStub{err: boom}
	svc := newLoginServiceForTest(t, issuer, auth, reauth.NewTokenReAuthenticator(verifier))

	result, err := svc.Reauthenticate(context.Background(), "access-token")

	require.Nil(t, result)
	require.ErrorIs(t, err, boom)
	require.Nil(t, issuer.captured)
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
