package authentication_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPasswordAuthStrategyWithLoginIdentityUsesPasswordCredentialV2(t *testing.T) {
	ctx := context.Background()
	loginIdentityID := meta.FromUint64(2001)
	userID := meta.FromUint64(1001)
	tenantID := meta.FromUint64(3001)
	identityRepo := newLoginIdentityRepoTestDouble(&authentication.LoginIdentityLookup{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		Provider:        loginidentity.ProviderUsername,
		Realm:           tenantID.String(),
		Identifier:      "zhangsan",
		Status:          loginidentity.StatusActive,
	})
	credRepo := &loginIdentityCredentialRepoTestDouble{
		passwordByLoginIdentity: map[meta.ID]credentialMaterial{
			loginIdentityID: {credentialID: meta.FromUint64(4001), material: "plainpep"},
		},
	}
	authenticator := authentication.NewAuthenticator(
		authentication.NewPasswordAuthStrategyWithLoginIdentity(credRepo, identityRepo, &hasherStub{pepper: "pep"}),
	)
	proof, err := authentication.NewPasswordCredential(authentication.PasswordProofSpec{
		TenantID: tenantID,
		Username: "zhangsan",
		Password: "plain",
	})
	require.NoError(t, err)

	decision, err := authenticator.Authenticate(ctx, proof)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, loginIdentityID, decision.LoginIdentityID)
	require.Equal(t, loginIdentityID, decision.Principal.LoginIdentityID)
	require.Equal(t, userID, decision.Principal.UserID)
	require.Equal(t, "password", decision.Principal.AuthMethod)
	require.Equal(t, tenantID.String(), decision.Principal.Realm)
	require.Equal(t, 1, credRepo.findByLoginIdentityCalls)
}

func TestPhoneOTPAuthStrategyWithLoginIdentityDoesNotRequireLongTermCredential(t *testing.T) {
	ctx := context.Background()
	loginIdentityID := meta.FromUint64(2001)
	userID := meta.FromUint64(1001)
	phone := "+8613811112222"
	identityRepo := newLoginIdentityRepoTestDouble(&authentication.LoginIdentityLookup{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		Provider:        loginidentity.ProviderPhone,
		Realm:           loginidentity.RealmGlobal,
		Identifier:      phone,
		Status:          loginidentity.StatusActive,
	})
	authenticator := authentication.NewAuthenticator(
		authentication.NewPhoneOTPAuthStrategyWithLoginIdentity(identityRepo, otpVerifierTestDouble{ok: true}),
	)
	proof, err := authentication.NewPhoneOTPCredential(authentication.PhoneOTPProofSpec{
		PhoneE164: phone,
		OTP:       "123456",
	})
	require.NoError(t, err)

	decision, err := authenticator.Authenticate(ctx, proof)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, loginIdentityID, decision.LoginIdentityID)
	require.True(t, decision.CredentialID.IsZero())
	require.Equal(t, "phone_otp", decision.Principal.AuthMethod)
	require.Equal(t, loginidentity.RealmGlobal, decision.Principal.Realm)
}

func TestWechatOpenAuthStrategyWithLoginIdentityFallsBackToUnionID(t *testing.T) {
	ctx := context.Background()
	loginIdentityID := meta.FromUint64(2001)
	userID := meta.FromUint64(1001)
	identityRepo := newLoginIdentityRepoTestDouble(&authentication.LoginIdentityLookup{
		LoginIdentityID:  loginIdentityID,
		UserID:           userID,
		Provider:         loginidentity.ProviderWechatOpen,
		Realm:            "wx-app",
		Identifier:       "openid-1",
		GlobalIdentifier: "union-1",
		Status:           loginidentity.StatusActive,
	})
	identityRepo.providerLookups = map[string]*authentication.LoginIdentityLookup{}
	authenticator := authentication.NewAuthenticator(
		authentication.NewOAuthWechatOpenAuthStrategyWithLoginIdentity(identityRepo, idpTestDouble{wxOpenID: "openid-1", wxUnionID: "union-1"}),
	)
	proof, err := authentication.NewWechatOpenCredential(authentication.WechatOpenProofSpec{
		AppID:     "wx-app",
		AppSecret: "secret",
		Code:      "code",
	})
	require.NoError(t, err)

	decision, err := authenticator.Authenticate(ctx, proof)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, loginIdentityID, decision.LoginIdentityID)
	require.True(t, decision.CredentialID.IsZero())
	require.Equal(t, "oauth_wx_open", decision.Principal.AuthMethod)
	require.Equal(t, "wx-app", decision.Principal.Realm)
	require.Equal(t, "openid-1", decision.Principal.Claims["wx_openid"])
	require.Equal(t, "union-1", decision.Principal.Claims["wx_unionid"])
}

func TestWechatOpenAuthStrategyWithLoginIdentityPrefersOpenIDOverUnionIDFallback(t *testing.T) {
	ctx := context.Background()
	openIDLoginIdentityID := meta.FromUint64(2001)
	unionIDLoginIdentityID := meta.FromUint64(2002)
	userID := meta.FromUint64(1001)
	identityRepo := newLoginIdentityRepoTestDouble(
		&authentication.LoginIdentityLookup{
			LoginIdentityID:  openIDLoginIdentityID,
			UserID:           userID,
			Provider:         loginidentity.ProviderWechatOpen,
			Realm:            "wx-app",
			Identifier:       "openid-1",
			GlobalIdentifier: "union-1",
			Status:           loginidentity.StatusActive,
		},
		&authentication.LoginIdentityLookup{
			LoginIdentityID:  unionIDLoginIdentityID,
			UserID:           meta.FromUint64(1002),
			Provider:         loginidentity.ProviderWechatOpen,
			Realm:            "wx-app",
			Identifier:       "openid-old",
			GlobalIdentifier: "union-1",
			Status:           loginidentity.StatusActive,
		},
	)
	authenticator := authentication.NewAuthenticator(
		authentication.NewOAuthWechatOpenAuthStrategyWithLoginIdentity(identityRepo, idpTestDouble{wxOpenID: "openid-1", wxUnionID: "union-1"}),
	)
	proof, err := authentication.NewWechatOpenCredential(authentication.WechatOpenProofSpec{
		AppID:     "wx-app",
		AppSecret: "secret",
		Code:      "code",
	})
	require.NoError(t, err)

	decision, err := authenticator.Authenticate(ctx, proof)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, openIDLoginIdentityID, decision.LoginIdentityID)
	require.Equal(t, openIDLoginIdentityID, decision.Principal.LoginIdentityID)
	require.Equal(t, userID, decision.Principal.UserID)
}

func TestWechatMinipAuthStrategyWithLoginIdentityFallsBackToUnionID(t *testing.T) {
	ctx := context.Background()
	loginIdentityID := meta.FromUint64(2001)
	userID := meta.FromUint64(1001)
	identityRepo := newLoginIdentityRepoTestDouble(&authentication.LoginIdentityLookup{
		LoginIdentityID:  loginIdentityID,
		UserID:           userID,
		Provider:         loginidentity.ProviderWechatMinip,
		Realm:            "wx-app",
		Identifier:       "openid-1",
		GlobalIdentifier: "union-1",
		Status:           loginidentity.StatusActive,
	})
	identityRepo.providerLookups = map[string]*authentication.LoginIdentityLookup{}
	authenticator := authentication.NewAuthenticator(
		authentication.NewOAuthWechatMinipAuthStrategyWithLoginIdentity(identityRepo, idpTestDouble{wxOpenID: "openid-1", wxUnionID: "union-1"}),
	)
	proof, err := authentication.NewWechatMiniCredential(authentication.WechatMiniProofSpec{
		AppID:     "wx-app",
		AppSecret: "secret",
		Code:      "code",
	})
	require.NoError(t, err)

	decision, err := authenticator.Authenticate(ctx, proof)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, loginIdentityID, decision.LoginIdentityID)
	require.True(t, decision.CredentialID.IsZero())
	require.Equal(t, "wechat_minip", decision.Principal.AuthMethod)
	require.Equal(t, "wx-app", decision.Principal.Realm)
}

func TestWecomAuthStrategyWithLoginIdentityDoesNotRequireLongTermCredential(t *testing.T) {
	ctx := context.Background()
	loginIdentityID := meta.FromUint64(2001)
	userID := meta.FromUint64(1001)
	identityRepo := newLoginIdentityRepoTestDouble(&authentication.LoginIdentityLookup{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		Provider:        loginidentity.ProviderWecom,
		Realm:           "corp-1",
		Identifier:      "user-1",
		Status:          loginidentity.StatusActive,
	})
	authenticator := authentication.NewAuthenticator(
		authentication.NewOAuthWeChatComAuthStrategyWithLoginIdentity(identityRepo, idpTestDouble{wecomOpenUserID: "open-user-1", wecomUserID: "user-1"}),
	)
	proof, err := authentication.NewWecomCredential(authentication.WecomProofSpec{
		CorpID:     "corp-1",
		AgentID:    "agent-1",
		CorpSecret: "secret",
		Code:       "code",
	})
	require.NoError(t, err)

	decision, err := authenticator.Authenticate(ctx, proof)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, loginIdentityID, decision.LoginIdentityID)
	require.True(t, decision.CredentialID.IsZero())
	require.Equal(t, "wecom", decision.Principal.AuthMethod)
	require.Equal(t, "corp-1", decision.Principal.Realm)
}

type credentialMaterial struct {
	credentialID meta.ID
	material     string
	disabled     bool
	lockedUntil  *time.Time
}

type loginIdentityCredentialRepoTestDouble struct {
	passwordByLoginIdentity  map[meta.ID]credentialMaterial
	findByLoginIdentityCalls int
}

func (s *loginIdentityCredentialRepoTestDouble) FindPasswordCredentialByLoginIdentity(_ context.Context, loginIdentityID meta.ID) (*authentication.PasswordCredentialLookup, error) {
	s.findByLoginIdentityCalls++
	material := s.passwordByLoginIdentity[loginIdentityID]
	if material.credentialID.IsZero() {
		return nil, nil
	}
	return &authentication.PasswordCredentialLookup{
		CredentialID: material.credentialID,
		PasswordHash: material.material,
		Status:       material.statusOrEnabled(),
		LockedUntil:  material.lockedUntil,
	}, nil
}

func (m credentialMaterial) statusOrEnabled() credDomain.CredentialStatus {
	if m.disabled {
		return credDomain.CredStatusDisabled
	}
	return credDomain.CredStatusEnabled
}

type loginIdentityRepoTestDouble struct {
	providerLookups map[string]*authentication.LoginIdentityLookup
	globalLookups   map[string]*authentication.LoginIdentityLookup
	statusByID      map[meta.ID]loginidentity.Status
}

func newLoginIdentityRepoTestDouble(lookups ...*authentication.LoginIdentityLookup) *loginIdentityRepoTestDouble {
	repo := &loginIdentityRepoTestDouble{
		providerLookups: map[string]*authentication.LoginIdentityLookup{},
		globalLookups:   map[string]*authentication.LoginIdentityLookup{},
		statusByID:      map[meta.ID]loginidentity.Status{},
	}
	for _, lookup := range lookups {
		repo.providerLookups[providerLookupKey(lookup.Provider, lookup.Realm, lookup.Identifier)] = lookup
		if lookup.GlobalIdentifier != "" {
			repo.globalLookups[globalLookupKey(lookup.Provider, lookup.GlobalIdentifier)] = lookup
		}
		repo.statusByID[lookup.LoginIdentityID] = lookup.Status
	}
	return repo
}

func (s *loginIdentityRepoTestDouble) FindUsernameIdentity(ctx context.Context, tenantID meta.ID, username string) (*authentication.LoginIdentityLookup, error) {
	return s.FindLoginIdentityByProviderKey(ctx, loginidentity.ProviderUsername, loginidentity.UsernameRealm(tenantID), username)
}
func (s *loginIdentityRepoTestDouble) FindLoginIdentityByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*authentication.LoginIdentityLookup, error) {
	return s.providerLookups[providerLookupKey(provider, realm, identifier)], nil
}
func (s *loginIdentityRepoTestDouble) FindLoginIdentityByGlobalIdentifier(_ context.Context, provider loginidentity.Provider, globalIdentifier string) (*authentication.LoginIdentityLookup, error) {
	return s.globalLookups[globalLookupKey(provider, globalIdentifier)], nil
}
func (s *loginIdentityRepoTestDouble) IsLoginIdentityActive(_ context.Context, loginIdentityID meta.ID) (bool, error) {
	status, ok := s.statusByID[loginIdentityID]
	if !ok {
		return false, nil
	}
	return status == loginidentity.StatusActive, nil
}

func providerLookupKey(provider loginidentity.Provider, realm, identifier string) string {
	return fmt.Sprintf("%s|%s|%s", provider, realm, identifier)
}

func globalLookupKey(provider loginidentity.Provider, globalIdentifier string) string {
	return fmt.Sprintf("%s|%s", provider, globalIdentifier)
}

type otpVerifierTestDouble struct {
	ok bool
}

func (s otpVerifierTestDouble) VerifyAndConsumeLoginPhoneOTP(context.Context, string, string) bool {
	return s.ok
}

type idpTestDouble struct {
	wxOpenID        string
	wxUnionID       string
	wecomOpenUserID string
	wecomUserID     string
}

func (s idpTestDouble) ExchangeWxMinipCode(context.Context, string, string, string) (string, string, error) {
	return s.wxOpenID, s.wxUnionID, nil
}

func (s idpTestDouble) ExchangeWxOpenCode(context.Context, string, string, string) (string, string, error) {
	return s.wxOpenID, s.wxUnionID, nil
}

func (s idpTestDouble) ExchangeWecomCode(context.Context, string, string, string, string) (string, string, error) {
	return s.wecomOpenUserID, s.wecomUserID, nil
}

var _ authentication.LoginIdentityCredentialRepository = (*loginIdentityCredentialRepoTestDouble)(nil)
var _ authentication.LoginIdentityRepository = (*loginIdentityRepoTestDouble)(nil)
var _ authentication.LoginPhoneOTPVerifier = otpVerifierTestDouble{}
var _ authentication.IdentityProvider = idpTestDouble{}
