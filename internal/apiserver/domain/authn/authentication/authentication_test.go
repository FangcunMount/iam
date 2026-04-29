package authentication

import (
	"testing"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
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

func TestAuthenticator_CreateStrategyMapping(t *testing.T) {
	t.Parallel()

	a := NewAuthenticator(nil, nil, nil, nil, nil)

	// Known scenarios should map to non-nil strategies
	assert.NotNil(t, a.createStrategy(AuthPassword))
	assert.NotNil(t, a.createStrategy(AuthPhoneOTP))
	assert.NotNil(t, a.createStrategy(AuthWxMinip))
	assert.NotNil(t, a.createStrategy(AuthWecom))

	// Unknown scenario should return nil
	assert.Nil(t, a.createStrategy(Scenario("unknown")))
}
