package authentication

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestProofConstructorsValidateRequiredFieldsAndMapScenario(t *testing.T) {
	t.Parallel()

	password, err := NewPasswordCredential(PasswordProofSpec{TenantID: meta.FromUint64(1), Username: "alice", Password: "secret"})
	require.NoError(t, err)
	require.Equal(t, AuthPassword, password.Scenario())
	_, err = NewPasswordCredential(PasswordProofSpec{})
	require.Error(t, err)

	phone, err := NewPhoneOTPCredential(PhoneOTPProofSpec{TenantID: meta.FromUint64(1), PhoneE164: "+8613800138000", OTP: "123456"})
	require.NoError(t, err)
	require.Equal(t, AuthPhoneOTP, phone.Scenario())
	_, err = NewPhoneOTPCredential(PhoneOTPProofSpec{})
	require.Error(t, err)

	wechat, err := NewWechatMiniCredential(WechatMiniProofSpec{
		TenantID:  meta.FromUint64(1),
		AppID:     "wx-app",
		AppSecret: "secret",
		Code:      "code",
	})
	require.NoError(t, err)
	require.Equal(t, AuthWxMinip, wechat.Scenario())
	_, err = NewWechatMiniCredential(WechatMiniProofSpec{})
	require.Error(t, err)

	wecom, err := NewWecomCredential(WecomProofSpec{
		TenantID:   meta.FromUint64(1),
		CorpID:     "corp",
		AgentID:    "agent",
		CorpSecret: "secret",
		Code:       "code",
	})
	require.NoError(t, err)
	require.Equal(t, AuthWecom, wecom.Scenario())
	_, err = NewWecomCredential(WecomProofSpec{})
	require.Error(t, err)
}

type authenticatorStrategyStub struct {
	kind   Scenario
	called bool
}

func (s *authenticatorStrategyStub) Kind() Scenario {
	return s.kind
}

func (s *authenticatorStrategyStub) Authenticate(context.Context, AuthCredential) (AuthDecision, error) {
	s.called = true
	return AuthDecision{
		OK: true,
		Principal: &Principal{
			UserID:    meta.FromUint64(1001),
			AccountID: meta.FromUint64(2002),
			TenantID:  meta.FromUint64(1),
		},
	}, nil
}

func TestAuthenticatorUsesInjectedStrategyMapping(t *testing.T) {
	t.Parallel()

	strategy := &authenticatorStrategyStub{kind: AuthPassword}
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
	require.Nil(t, a.strategyFor(Scenario("unknown")))
}
