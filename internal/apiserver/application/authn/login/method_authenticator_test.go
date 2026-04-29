package login

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestMethodProofBuildersMapSelectedPayloads(t *testing.T) {
	t.Parallel()

	passwordProof, err := buildPasswordProof(PasswordPayload{
		methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(42)},
		Username:            "alice",
		Password:            "secret",
	})
	require.NoError(t, err)
	password, ok := passwordProof.(*authentication.PasswordCredential)
	require.True(t, ok)
	require.Equal(t, authentication.AuthPassword, password.Scenario())
	require.Equal(t, uint64(42), password.TenantID.Uint64())
	require.Equal(t, "alice", password.Username)

	phoneProof, err := buildPhoneOTPProof(PhoneOTPPayload{
		methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(7)},
		PhoneE164:           "+8613800138000",
		OTP:                 "123456",
	})
	require.NoError(t, err)
	phone, ok := phoneProof.(*authentication.PhoneOTPCredential)
	require.True(t, ok)
	require.Equal(t, authentication.AuthPhoneOTP, phone.Scenario())
	require.Equal(t, "+8613800138000", phone.PhoneE164)
}

func TestWecomMethodCurrentlyFailsBeforeDomainWhenAppSecretIsMissing(t *testing.T) {
	t.Parallel()

	router := newMethodAuthenticatorRouter(authentication.NewAuthenticator(), nil, nil, nil)
	decision, err := router.Authenticate(context.Background(), SelectedMethod{
		Method: MethodWecom,
		Payload: WecomPayload{
			methodPayloadCommon: methodPayloadCommon{TenantID: meta.FromUint64(1)},
			CorpID:              "corp-id",
			Code:                "auth-code",
		},
	})

	require.False(t, decision.OK)
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}
