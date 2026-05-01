package login

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	idpWechatApp "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestMethodProofPreparersMapPayloads(t *testing.T) {
	t.Parallel()

	passwordProof, err := newPasswordAdapter().PrepareProof(context.Background(), PasswordPayload{
		methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(42)},
		Username:            "alice",
		Password:            "secret",
	})
	require.NoError(t, err)
	password, ok := passwordProof.(*authentication.PasswordCredential)
	require.True(t, ok)
	require.Equal(t, credDomain.CredPassword, password.CredentialType())
	require.Equal(t, uint64(42), password.TenantID.Uint64())
	require.Equal(t, "alice", password.Username)

	phoneProof, err := newPhoneOTPAdapter().PrepareProof(context.Background(), PhoneOTPPayload{
		methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(7)},
		PhoneE164:           "+8613800138000",
		OTP:                 "123456",
	})
	require.NoError(t, err)
	phone, ok := phoneProof.(*authentication.PhoneOTPCredential)
	require.True(t, ok)
	require.Equal(t, credDomain.CredPhoneOTP, phone.CredentialType())
	require.Equal(t, "+8613800138000", phone.PhoneE164)
}

func TestWecomMethodFailsWhenServerAgentIDIsMissing(t *testing.T) {
	t.Parallel()

	adapter := newWecomAdapter(
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
	)
	credential, err := adapter.PrepareProof(context.Background(), WecomPayload{
		methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(1)},
		CorpID:              "corp-id",
		Code:                "auth-code",
	})

	require.Nil(t, credential)
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}

func TestWecomMethodAppConfigErrorBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		repo        *wecomAppRepoStub
		vault       wecomSecretVaultStub
		wantErrCode int
	}{
		{
			name:        "query failure",
			repo:        &wecomAppRepoStub{err: errors.New("db down")},
			vault:       wecomSecretVaultStub{plaintext: "corp-secret"},
			wantErrCode: code.ErrInvalidArgument,
		},
		{
			name:        "app missing",
			repo:        &wecomAppRepoStub{},
			vault:       wecomSecretVaultStub{plaintext: "corp-secret"},
			wantErrCode: code.ErrInvalidArgument,
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
			vault:       wecomSecretVaultStub{plaintext: "corp-secret"},
			wantErrCode: code.ErrInvalidArgument,
		},
		{
			name: "credentials missing",
			repo: &wecomAppRepoStub{app: &idpWechatApp.WechatApp{
				AppID:  "corp-id",
				Status: idpWechatApp.StatusEnabled,
			}},
			vault:       wecomSecretVaultStub{plaintext: "corp-secret"},
			wantErrCode: code.ErrInvalidArgument,
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
			vault:       wecomSecretVaultStub{err: errors.New("kms down")},
			wantErrCode: code.ErrInvalidArgument,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			adapter := newWecomAdapter(
				tc.repo,
				tc.vault,
				WecomConfig{AgentID: "agent-id"},
			)
			credential, err := adapter.PrepareProof(context.Background(), WecomPayload{
				methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(1)},
				CorpID:              "corp-id",
				Code:                "auth-code",
			})

			require.Nil(t, credential)
			require.Error(t, err)
			require.Equal(t, tc.wantErrCode, perrors.ParseCoder(err).Code())
		})
	}
}

func TestWecomMethodUsesServerSideAppConfigAndAuthenticates(t *testing.T) {
	t.Parallel()

	credRepo := &wecomCredentialRepoStub{
		accountID:    meta.FromUint64(1001),
		userID:       meta.FromUint64(2002),
		credentialID: meta.FromUint64(3003),
	}
	accountRepo := &loginAccountRepoStub{enabled: true}
	idp := &wecomIdentityProviderStub{
		openUserID: "open-user-id",
		userID:     "wecom-user-id",
	}
	auth := authentication.NewAuthenticator(authentication.NewOAuthWeChatComAuthStrategy(credRepo, accountRepo, idp))
	adapter := newWecomAdapter(
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
	)

	credential, err := adapter.PrepareProof(context.Background(), WecomPayload{
		methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(1)},
		CorpID:              "corp-id",
		Code:                "auth-code",
	})
	require.NoError(t, err)

	decision, err := auth.Authenticate(context.Background(), credential)
	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Equal(t, meta.FromUint64(1001), decision.Principal.AccountID)
	require.Equal(t, meta.FromUint64(2002), decision.Principal.UserID)
	require.Equal(t, meta.FromUint64(3003), decision.CredentialID)
	require.Equal(t, "corp-id", idp.corpID)
	require.Equal(t, "agent-id", idp.agentID)
	require.Equal(t, "corp-secret", idp.corpSecret)
	require.Equal(t, "auth-code", idp.code)
	require.Equal(t, "wecom", credRepo.idpType)
	require.Equal(t, "corp-id", credRepo.appID)
	require.Equal(t, "wecom-user-id", credRepo.idpIdentifier)
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

type wecomCredentialRepoStub struct {
	accountID     meta.ID
	userID        meta.ID
	credentialID  meta.ID
	idpType       string
	appID         string
	idpIdentifier string
}

func (s *wecomCredentialRepoStub) FindPasswordCredential(context.Context, meta.ID) (meta.ID, string, error) {
	return meta.ZeroID, "", nil
}

func (s *wecomCredentialRepoStub) FindPhoneOTPCredential(context.Context, string) (meta.ID, meta.ID, meta.ID, error) {
	return meta.ZeroID, meta.ZeroID, meta.ZeroID, nil
}

func (s *wecomCredentialRepoStub) FindOAuthCredential(_ context.Context, idpType, appID, idpIdentifier string) (meta.ID, meta.ID, meta.ID, error) {
	s.idpType = idpType
	s.appID = appID
	s.idpIdentifier = idpIdentifier
	return s.accountID, s.userID, s.credentialID, nil
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

func (s *wecomIdentityProviderStub) ExchangeWecomCode(_ context.Context, corpID, agentID, corpSecret, code string) (string, string, error) {
	s.corpID = corpID
	s.agentID = agentID
	s.corpSecret = corpSecret
	s.code = code
	return s.openUserID, s.userID, s.err
}
