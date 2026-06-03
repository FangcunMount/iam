package proof

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	idpWechatApp "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestMethodProofPreparersMapPayloads(t *testing.T) {
	t.Parallel()

	passwordProof, err := NewPasswordBuilder().Build(context.Background(), method.PasswordPayload{
		Username: "alice",
		Password: "secret",
	}, method.CommonPayload{TenantID: meta.FromUint64(42)})
	require.NoError(t, err)
	password, ok := passwordProof.(*authentication.PasswordCredential)
	require.True(t, ok)
	require.Equal(t, authentication.CredentialKindPassword, password.CredentialKind())
	require.Equal(t, uint64(42), password.TenantID.Uint64())
	require.Equal(t, "alice", password.Username)

	phoneProof, err := NewPhoneOTPBuilder().Build(context.Background(), method.PhoneOTPPayload{
		PhoneE164: "+8613800138000",
		OTP:       "123456",
	}, method.CommonPayload{TenantID: meta.FromUint64(7)})
	require.NoError(t, err)
	phone, ok := phoneProof.(*authentication.PhoneOTPCredential)
	require.True(t, ok)
	require.Equal(t, authentication.CredentialKindPhoneOTP, phone.CredentialKind())
	require.Equal(t, "+8613800138000", phone.PhoneE164)
}

func TestWecomMethodFailsWhenServerAgentIDIsMissing(t *testing.T) {
	t.Parallel()

	factory := DefaultFactory(
		&wecomAppRepoStub{
			app: &idpWechatApp.WechatApp{
				AppID:  "corp-id",
				Status: idpWechatApp.StatusEnabled,
				Cred: &idpWechatApp.Credentials{
					Auth: &idpWechatApp.AuthSecret{AppSecretCipher: []byte("cipher")},
				},
			},
		},
		wecomSecretVaultStub{plaintext: "corp-secret"},
		WecomConfig{},
		nil,
	)
	credential, err := factory.Build(context.Background(), method.LoginMethodSelection{
		CredentialKind: method.CredentialKindWecom,
		Common:         method.CommonPayload{TenantID: meta.FromUint64(1)},
		Payload: method.WecomPayload{
			CorpID: "corp-id",
			Code:   "auth-code",
		},
	})

	require.Nil(t, credential)
	require.Error(t, err)
	require.Equal(t, code.ErrProofBuildFailed, perrors.ParseCoder(err).Code())
}

func TestWecomMethodAppConfigErrorBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		repo     *wecomAppRepoStub
		vault    wecomSecretVaultStub
		wantCode int
	}{
		{
			name:     "query failure",
			repo:     &wecomAppRepoStub{err: errors.New("db down")},
			vault:    wecomSecretVaultStub{plaintext: "corp-secret"},
			wantCode: code.ErrProofBuildFailed,
		},
		{
			name:     "app missing",
			repo:     &wecomAppRepoStub{},
			vault:    wecomSecretVaultStub{plaintext: "corp-secret"},
			wantCode: code.ErrProofBuildFailed,
		},
		{
			name: "app disabled",
			repo: &wecomAppRepoStub{app: &idpWechatApp.WechatApp{
				AppID:  "corp-id",
				Status: idpWechatApp.StatusDisabled,
				Cred: &idpWechatApp.Credentials{
					Auth: &idpWechatApp.AuthSecret{AppSecretCipher: []byte("cipher")},
				},
			}},
			vault:    wecomSecretVaultStub{plaintext: "corp-secret"},
			wantCode: code.ErrProofBuildFailed,
		},
		{
			name: "credentials missing",
			repo: &wecomAppRepoStub{app: &idpWechatApp.WechatApp{
				AppID:  "corp-id",
				Status: idpWechatApp.StatusEnabled,
			}},
			vault:    wecomSecretVaultStub{plaintext: "corp-secret"},
			wantCode: code.ErrProofBuildFailed,
		},
		{
			name: "secret decrypt failure",
			repo: &wecomAppRepoStub{app: &idpWechatApp.WechatApp{
				AppID:  "corp-id",
				Status: idpWechatApp.StatusEnabled,
				Cred: &idpWechatApp.Credentials{
					Auth: &idpWechatApp.AuthSecret{AppSecretCipher: []byte("cipher")},
				},
			}},
			vault:    wecomSecretVaultStub{err: errors.New("kms down")},
			wantCode: code.ErrProofBuildFailed,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			factory := DefaultFactory(
				tc.repo,
				tc.vault,
				WecomConfig{AgentID: "agent-id"},
				nil,
			)
			credential, err := factory.Build(context.Background(), method.LoginMethodSelection{
				CredentialKind: method.CredentialKindWecom,
				Common:         method.CommonPayload{TenantID: meta.FromUint64(1)},
				Payload: method.WecomPayload{
					CorpID: "corp-id",
					Code:   "auth-code",
				},
			})

			require.Nil(t, credential)
			require.Error(t, err)
			require.Equal(t, tc.wantCode, perrors.ParseCoder(err).Code())
		})
	}
}

func TestWecomMethodUsesServerSideAppConfigAndAuthenticates(t *testing.T) {
	t.Parallel()

	loginIdentityID := meta.FromUint64(1001)
	userID := meta.FromUint64(2002)
	identityRepo := &wecomLoginIdentityRepoStub{
		lookup: &authentication.LoginIdentityLookup{
			LoginIdentityID: loginIdentityID,
			UserID:          userID,
			Provider:        loginidentity.ProviderWecom,
			Realm:           "corp-id",
			Identifier:      "wecom-user-id",
			Status:          loginidentity.StatusActive,
		},
	}
	idp := &wecomIdentityProviderStub{
		openUserID: "open-user-id",
		userID:     "wecom-user-id",
	}
	auth := authentication.NewAuthenticator(authentication.NewOAuthWeChatComAuthStrategyWithLoginIdentity(identityRepo, idp))
	factory := DefaultFactory(
		&wecomAppRepoStub{
			app: &idpWechatApp.WechatApp{
				AppID:  "corp-id",
				Status: idpWechatApp.StatusEnabled,
				Cred: &idpWechatApp.Credentials{
					Auth: &idpWechatApp.AuthSecret{AppSecretCipher: []byte("cipher")},
				},
			},
		},
		wecomSecretVaultStub{plaintext: "corp-secret"},
		WecomConfig{AgentID: "agent-id"},
		nil,
	)

	credential, err := factory.Build(context.Background(), method.LoginMethodSelection{
		CredentialKind: method.CredentialKindWecom,
		Common:         method.CommonPayload{TenantID: meta.FromUint64(1)},
		Payload: method.WecomPayload{
			CorpID: "corp-id",
			Code:   "auth-code",
		},
	})
	require.NoError(t, err)

	decision, err := auth.Authenticate(context.Background(), credential)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, loginIdentityID, decision.Principal.LoginIdentityID)
	require.Equal(t, userID, decision.Principal.UserID)
	require.True(t, decision.CredentialID.IsZero())
	require.Equal(t, "corp-id", idp.corpID)
	require.Equal(t, "agent-id", idp.agentID)
	require.Equal(t, "corp-secret", idp.corpSecret)
	require.Equal(t, "auth-code", idp.code)
	require.Equal(t, loginidentity.ProviderWecom, identityRepo.provider)
	require.Equal(t, "corp-id", identityRepo.realm)
	require.Equal(t, "wecom-user-id", identityRepo.identifier)
}

type wecomAppRepoStub struct {
	app *idpWechatApp.WechatApp
	err error
}

func (s *wecomAppRepoStub) Create(context.Context, *idpWechatApp.WechatApp) error {
	return nil
}

func (s *wecomAppRepoStub) GetByID(context.Context, idutil.ID) (*idpWechatApp.WechatApp, error) {
	return nil, nil
}

func (s *wecomAppRepoStub) GetByAppID(context.Context, string) (*idpWechatApp.WechatApp, error) {
	return s.app, s.err
}

func (s *wecomAppRepoStub) List(context.Context, idpWechatApp.ListFilter) ([]*idpWechatApp.WechatApp, error) {
	return nil, nil
}

func (s *wecomAppRepoStub) Update(context.Context, *idpWechatApp.WechatApp) error {
	return nil
}

type wecomSecretVaultStub struct {
	plaintext string
	err       error
}

func (s wecomSecretVaultStub) Encrypt(context.Context, []byte) ([]byte, error) {
	return nil, nil
}

func (s wecomSecretVaultStub) Decrypt(context.Context, []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []byte(s.plaintext), nil
}

func (s wecomSecretVaultStub) Sign(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

type wecomIdentityProviderStub struct {
	corpID     string
	agentID    string
	corpSecret string
	code       string
	openUserID string
	userID     string
	err        error
}

func (s *wecomIdentityProviderStub) ExchangeWxMinipCode(context.Context, string, string, string) (string, string, error) {
	return "", "", nil
}

func (s *wecomIdentityProviderStub) ExchangeWxOpenCode(context.Context, string, string, string) (string, string, error) {
	return "", "", nil
}

func (s *wecomIdentityProviderStub) ExchangeWecomCode(_ context.Context, corpID, agentID, corpSecret, code string) (string, string, error) {
	s.corpID = corpID
	s.agentID = agentID
	s.corpSecret = corpSecret
	s.code = code
	return s.openUserID, s.userID, s.err
}

type wecomLoginIdentityRepoStub struct {
	lookup     *authentication.LoginIdentityLookup
	provider   loginidentity.Provider
	realm      string
	identifier string
}

func (s *wecomLoginIdentityRepoStub) FindUsernameIdentity(context.Context, meta.ID, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *wecomLoginIdentityRepoStub) FindLoginIdentityByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*authentication.LoginIdentityLookup, error) {
	s.provider = provider
	s.realm = realm
	s.identifier = identifier
	if s.lookup == nil || s.lookup.Provider != provider || s.lookup.Realm != realm || s.lookup.Identifier != identifier {
		return nil, nil
	}
	return s.lookup, nil
}

func (s *wecomLoginIdentityRepoStub) FindLoginIdentityByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *wecomLoginIdentityRepoStub) IsLoginIdentityActive(context.Context, meta.ID) (bool, error) {
	return true, nil
}
