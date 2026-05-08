package method

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type fakePayload struct {
	value string
}

func (fakePayload) loginMethodPayload() {}

type fakeLoginMethod struct {
	credentialKind CredentialKind
	authMethod     AuthMethod
	build          func(LoginRequest) (Payload, error)
}

func (m fakeLoginMethod) CredentialKind() CredentialKind {
	return m.credentialKind
}

func (m fakeLoginMethod) Method() AuthMethod {
	return m.authMethod
}

func (m fakeLoginMethod) BuildPayload(cmd LoginRequest) (Payload, error) {
	if m.build == nil {
		return fakePayload{value: "default"}, nil
	}
	return m.build(cmd)
}

func TestSelectorRejectsDuplicateCredentialKindAndAuthMethod(t *testing.T) {
	t.Parallel()

	_, err := NewSelector(
		fakeLoginMethod{credentialKind: "fake", authMethod: "fake_a"},
		fakeLoginMethod{credentialKind: "fake", authMethod: "fake_b"},
	)
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())

	_, err = NewSelector(
		fakeLoginMethod{credentialKind: "fake_a", authMethod: "fake"},
		fakeLoginMethod{credentialKind: "fake_b", authMethod: "fake"},
	)
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}

func TestSelectorUsesAuthMethodAsAuthority(t *testing.T) {
	t.Parallel()

	selected, err := DefaultSelector().Select(context.Background(), LoginRequest{
		AuthMethod: AuthMethodPassword,
		Payload: PhoneOTPPayload{
			PhoneE164: "+8613800138000",
			OTP:       "123456",
		},
	})

	require.Error(t, err)
	require.Equal(t, code.ErrPayloadInvalid, perrors.ParseCoder(err).Code())
	require.Equal(t, LoginMethodSelection{}, selected)
}

func TestSelectorBuildsSelectedMethodPayload(t *testing.T) {
	t.Parallel()

	selected, err := DefaultSelector().Select(context.Background(), LoginRequest{
		AuthMethod: AuthMethodPassword,
		TenantID:   meta.FromUint64(77),
		RemoteIP:   "127.0.0.1",
		UserAgent:  "iam-test",
		Payload: PasswordPayload{
			Username: "alice",
			Password: "secret",
		},
	})

	require.NoError(t, err)
	require.Equal(t, AuthMethodPassword, selected.AuthMethod)
	require.Equal(t, CredentialKindPassword, selected.CredentialKind)
	payload, ok := selected.Payload.(PasswordPayload)
	require.True(t, ok)
	require.Equal(t, "alice", payload.Username)
	require.Equal(t, "secret", payload.Password)
	require.Equal(t, uint64(77), selected.Common.TenantID.Uint64())
	require.Equal(t, "127.0.0.1", selected.Common.RemoteIP)
	require.Equal(t, "iam-test", selected.Common.UserAgent)
}

func TestSelectorRequiresSelectedMethodFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  LoginRequest
	}{
		{name: "password", cmd: LoginRequest{AuthMethod: AuthMethodPassword}},
		{name: "phone", cmd: LoginRequest{AuthMethod: AuthMethodPhoneOTP}},
		{name: "wechat", cmd: LoginRequest{AuthMethod: AuthMethodWechat}},
		{name: "wecom", cmd: LoginRequest{AuthMethod: AuthMethodWecom}},
	}

	selector := DefaultSelector()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := selector.Select(context.Background(), tc.cmd)

			require.Error(t, err)
			require.Equal(t, code.ErrPayloadInvalid, perrors.ParseCoder(err).Code())
		})
	}
}
