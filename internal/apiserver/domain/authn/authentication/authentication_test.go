package authentication

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestProofConstructorsValidateRequiredFieldsAndMapCredentialKind(t *testing.T) {
	t.Parallel()

	password, err := NewPasswordCredential(PasswordProofSpec{TenantID: meta.FromUint64(1), Username: "alice", Password: "secret"})
	require.NoError(t, err)
	require.Equal(t, CredentialKindPassword, password.CredentialKind())
	_, err = NewPasswordCredential(PasswordProofSpec{})
	require.Error(t, err)

	phone, err := NewPhoneOTPCredential(PhoneOTPProofSpec{TenantID: meta.FromUint64(1), PhoneE164: "+8613800138000", OTP: "123456"})
	require.NoError(t, err)
	require.Equal(t, CredentialKindPhoneOTP, phone.CredentialKind())
	_, err = NewPhoneOTPCredential(PhoneOTPProofSpec{})
	require.Error(t, err)

	wechat, err := NewWechatMiniCredential(WechatMiniProofSpec{
		TenantID: meta.FromUint64(1),
		AppID:    "wx-app",
		OpenID:   "open-id",
	})
	require.NoError(t, err)
	require.Equal(t, CredentialKindWechatMinip, wechat.CredentialKind())
	_, err = NewWechatMiniCredential(WechatMiniProofSpec{})
	require.Error(t, err)

	wechatOpen, err := NewWechatOpenCredential(WechatOpenProofSpec{
		AppID:  "wx-app",
		OpenID: "open-id",
	})
	require.NoError(t, err)
	require.Equal(t, CredentialKindWechatOpen, wechatOpen.CredentialKind())
	_, err = NewWechatOpenCredential(WechatOpenProofSpec{})
	require.Error(t, err)

	wecom, err := NewWecomCredential(WecomProofSpec{
		TenantID: meta.FromUint64(1),
		CorpID:   "corp",
		UserID:   "user-id",
	})
	require.NoError(t, err)
	require.Equal(t, CredentialKindWecom, wecom.CredentialKind())
	_, err = NewWecomCredential(WecomProofSpec{})
	require.Error(t, err)
}

type authenticatorStrategyStub struct {
	kind        CredentialKind
	called      bool
	hasDecision bool
	decision    AuthDecision
	err         error
}

func (s *authenticatorStrategyStub) Kind() CredentialKind {
	return s.kind
}

func (s *authenticatorStrategyStub) Authenticate(context.Context, AuthCredential) (AuthDecision, error) {
	s.called = true
	if s.err != nil {
		return AuthDecision{}, s.err
	}
	if s.hasDecision {
		return s.decision, nil
	}
	return AuthDecision{
		OK: true,
		Principal: &Principal{
			UserID:          meta.FromUint64(1001),
			LoginIdentityID: meta.FromUint64(2002),
			TenantID:        meta.FromUint64(1),
		},
	}, nil
}

func TestAuthenticatorUsesInjectedStrategyMapping(t *testing.T) {
	t.Parallel()

	strategy := &authenticatorStrategyStub{kind: CredentialKindPassword}
	a := NewAuthenticator(strategy)
	proof, err := NewPasswordCredential(PasswordProofSpec{
		TenantID: meta.FromUint64(1),
		Username: "alice",
		Password: "secret",
	})
	require.NoError(t, err)

	decision, err := a.Authenticate(context.Background(), proof)

	require.NoError(t, err)
	require.True(t, decision.OK)
	require.True(t, strategy.called)
	require.Nil(t, a.strategyFor(CredentialKind("unknown")))
}
