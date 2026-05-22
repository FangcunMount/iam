package login

import (
	"context"
	"errors"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/proof"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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

func (s *loginTokenIssuerStub) IssueServiceToken(ctx context.Context, req tokenapp.IssueServiceTokenRequest) (*tokenapp.TokenIssueResult, error) {
	return nil, nil
}

func (s *loginTokenIssuerStub) RefreshToken(ctx context.Context, refreshToken string) (*tokenapp.TokenRefreshResult, error) {
	return nil, nil
}

func (s *loginTokenIssuerStub) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	return nil
}

func (s *loginTokenIssuerStub) RevokeRefreshToken(ctx context.Context, tokenValue string) error {
	return nil
}

func (s *loginTokenIssuerStub) VerifyToken(ctx context.Context, req tokenapp.VerifyTokenRequest) (*tokenapp.TokenVerifyResult, error) {
	return nil, nil
}

type loginTokenVerifierStub struct {
	userID          meta.ID
	loginIdentityID meta.ID
	tenantDomain    string
	sessionID       string
	amr             []string
	attrs           map[string]string
	token           string
	err             error
	code            int
}

func (s *loginTokenVerifierStub) VerifyToken(ctx context.Context, req tokenapp.VerifyTokenRequest) (*tokenapp.TokenVerifyResult, error) {
	s.token = req.AccessToken
	if s.err != nil {
		return nil, s.err
	}
	if s.code != 0 {
		return &tokenapp.TokenVerifyResult{Valid: false, FailureCode: s.code}, nil
	}
	return &tokenapp.TokenVerifyResult{
		Valid: true,
		Claims: &tokenapp.TokenClaims{
			UserID:          s.userID,
			LoginIdentityID: s.loginIdentityID,
			TenantDomain:    s.tenantDomain,
			SessionID:       s.sessionID,
			AMR:             s.amr,
			Attributes:      s.attrs,
		},
	}, nil
}

func newLoginServiceForTest(t *testing.T, tokenService tokenapp.TokenApplicationService, auth *authentication.Authenticator, reAuthenticator ReAuthenticator) LoginApplicationService {
	t.Helper()

	svc, err := NewLoginApplicationService(Dependencies{
		TokenService:    tokenService,
		Authenticator:   auth,
		MethodRegistry:  method.DefaultSelector(),
		ProofFactory:    proof.DefaultFactory(nil, nil, WecomConfig{}),
		ReAuthenticator: reAuthenticator,
	})
	require.NoError(t, err)
	return svc
}

func TestReauthenticateDefaultsMissingTenantDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tokenDomain string
		wantDomain  string
	}{
		{
			name:        "fills default tenant domain when missing",
			tokenDomain: "",
			wantDomain:  "fangcun",
		},
		{
			name:        "keeps explicit tenant domain",
			tokenDomain: "platform",
			wantDomain:  "platform",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			auth := authentication.NewAuthenticator()

			issuer := &loginTokenIssuerStub{}
			verifier := &loginTokenVerifierStub{
				userID:          meta.FromUint64(1001),
				loginIdentityID: meta.FromUint64(2002),
				tenantDomain:    tc.tokenDomain,
				sessionID:       "session-id",
				amr:             []string{"pwd"},
				attrs:           map[string]string{"scope": "profile"},
			}
			svc := newLoginServiceForTest(t, issuer, auth, NewTokenReAuthenticator(verifier))

			result, err := svc.Reauthenticate(context.Background(), " access-token-value ")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Principal)

			require.Equal(t, tc.wantDomain, result.Principal.Claims["tenant_domain"])
			require.Equal(t, meta.FromUint64(1001), result.UserID)
			require.Equal(t, meta.FromUint64(2002), result.LoginIdentityID)
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
	verifier := &loginTokenVerifierStub{code: code.ErrExpired}
	svc := newLoginServiceForTest(t, issuer, auth, NewTokenReAuthenticator(verifier))

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
	svc := newLoginServiceForTest(t, issuer, auth, NewTokenReAuthenticator(verifier))

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

func TestLoginPayloadFailureUsesPayloadInvalidCode(t *testing.T) {
	t.Parallel()

	auth := authentication.NewAuthenticator()
	issuer := &loginTokenIssuerStub{}
	svc := newLoginServiceForTest(t, issuer, auth, NewTokenReAuthenticator(&loginTokenVerifierStub{}))

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
	require.Nil(t, issuer.captured)
}

func TestLoginProofFailureUsesProofBuildFailedCode(t *testing.T) {
	t.Parallel()

	auth := authentication.NewAuthenticator()
	issuer := &loginTokenIssuerStub{}
	svc := newLoginServiceForTest(t, issuer, auth, NewTokenReAuthenticator(&loginTokenVerifierStub{}))

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
	require.Nil(t, issuer.captured)
}
