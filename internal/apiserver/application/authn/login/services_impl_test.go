package login

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	domaintoken "github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/FangcunMount/iam/pkg/tenant"
	"github.com/stretchr/testify/require"
)

type loginTokenIssuerStub struct {
	captured *authentication.Principal
}

func (s *loginTokenIssuerStub) IssueToken(ctx context.Context, principal *authentication.Principal) (*domaintoken.TokenPair, error) {
	s.captured = principal
	access := domaintoken.NewAccessToken(
		"access-id",
		"access-value",
		"session-id",
		principal.UserID,
		principal.AccountID,
		principal.TenantID,
		time.Minute,
	)
	refresh := domaintoken.NewRefreshToken(
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
	return domaintoken.NewTokenPair(access, refresh), nil
}

func (s *loginTokenIssuerStub) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*domaintoken.TokenPair, error) {
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
