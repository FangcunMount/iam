package login

import (
	"context"
	"testing"
	"time"

	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/FangcunMount/iam/pkg/tenant"
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
}

func (s *loginTokenVerifierStub) VerifyAccessToken(ctx context.Context, tokenValue string) (userID, accountID, tenantID meta.ID, err error) {
	return s.userID, s.accountID, s.tenantID, nil
}

func TestLogin_DefaultsMissingTenantIDBeforeTokenIssue(t *testing.T) {
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

			auth := authentication.NewAuthenticater(
				nil,
				&loginAccountRepoStub{enabled: true},
				nil,
				nil,
				nil,
				&loginTokenVerifierStub{
					userID:    meta.FromUint64(1001),
					accountID: meta.FromUint64(2002),
					tenantID:  tc.tokenTenant,
				},
			)

			issuer := &loginTokenIssuerStub{}
			svc := NewLoginApplicationService(issuer, nil, auth, nil, nil)

			jwtToken := "jwt-token-value"
			result, err := svc.Login(context.Background(), LoginRequest{
				AuthType: AuthTypeJWTToken,
				JWTToken: &jwtToken,
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Principal)
			require.NotNil(t, result.TokenPair)
			require.NotNil(t, result.TokenPair.AccessToken)
			require.NotNil(t, issuer.captured)

			require.Equal(t, tc.wantTenant, result.TenantID.Uint64())
			require.Equal(t, tc.wantTenant, result.Principal.TenantID.Uint64())
			require.Equal(t, tc.wantTenant, issuer.captured.TenantID.Uint64())
			require.Equal(t, tc.wantTenant, result.TokenPair.AccessToken.TenantID.Uint64())
			require.Equal(t, tc.wantTenant, result.TokenPair.RefreshToken.TenantID.Uint64())
		})
	}
}

func TestPrepareAuthenticationCharacterizesCurrentFieldInferencePrecedence(t *testing.T) {
	t.Parallel()

	username := "alice"
	password := "secret"
	phone := "+8613800138000"
	otp := "123456"
	jwtToken := "jwt-token"

	tests := []struct {
		name string
		req  LoginRequest
		want authentication.Scenario
	}{
		{
			name: "auth type is not authoritative when only password fields are present",
			req: LoginRequest{
				AuthType: AuthTypeJWTToken,
				Username: &username,
				Password: &password,
			},
			want: authentication.AuthPassword,
		},
		{
			name: "phone otp fields override password fields",
			req: LoginRequest{
				AuthType:  AuthTypePassword,
				Username:  &username,
				Password:  &password,
				PhoneE164: &phone,
				OTPCode:   &otp,
			},
			want: authentication.AuthPhoneOTP,
		},
		{
			name: "jwt token field wins over earlier credential fields",
			req: LoginRequest{
				AuthType:  AuthTypePassword,
				Username:  &username,
				Password:  &password,
				PhoneE164: &phone,
				OTPCode:   &otp,
				JWTToken:  &jwtToken,
			},
			want: authentication.AuthBearerToken,
		},
	}

	svc := &loginApplicationService{}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			selected, err := svc.selectScenario(context.Background(), tc.req)
			require.NoError(t, err)
			require.Equal(t, tc.want, selected.Scenario)
		})
	}
}

func TestExplicitScenarioSelectorUsesAuthTypeAsAuthority(t *testing.T) {
	t.Parallel()

	username := "alice"
	password := "secret"
	phone := "+8613800138000"
	otp := "123456"

	selector := newDefaultScenarioSelector()
	selected, err := selector.Select(context.Background(), LoginRequest{
		SelectionMode: ScenarioSelectionExplicit,
		AuthType:      AuthTypePassword,
		Username:      &username,
		Password:      &password,
		PhoneE164:     &phone,
		OTPCode:       &otp,
	})

	require.NoError(t, err)
	require.Equal(t, authentication.AuthPassword, selected.Scenario)
	require.Equal(t, username, selected.Input.Username)
	require.Equal(t, password, selected.Input.Password)
	require.Empty(t, selected.Input.PhoneE164)
	require.Empty(t, selected.Input.OTP)
}

func TestLegacyScenarioSelectorKeepsFieldInferenceWhenAuthTypeConflicts(t *testing.T) {
	t.Parallel()

	username := "alice"
	password := "secret"

	selector := newDefaultScenarioSelector()
	selected, err := selector.Select(context.Background(), LoginRequest{
		AuthType: AuthTypeJWTToken,
		Username: &username,
		Password: &password,
		JWTToken: nil,
		TenantID: meta.FromUint64(42),
	})

	require.NoError(t, err)
	require.Equal(t, authentication.AuthPassword, selected.Scenario)
	require.Equal(t, uint64(42), selected.Input.TenantID.Uint64())
}
