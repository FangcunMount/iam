package authfailure

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestMessageCoversKnownAuthnCodes(t *testing.T) {
	t.Parallel()

	tests := map[int]string{
		code.ErrUnauthenticated:       "authentication failed",
		code.ErrInvalidCredentials:    "invalid credentials",
		code.ErrTokenInvalid:          "token invalid",
		code.ErrSignatureInvalid:      "signature invalid",
		code.ErrExpired:               "token expired",
		code.ErrUserNotRegistered:     "user not registered",
		code.ErrLoginIdentityDisabled: "login identity is disabled",
		code.ErrCredentialLocked:      "credential is locked",
		code.ErrCredentialExpired:     "credential has expired",
		code.ErrCredentialDisabled:    "credential is disabled",
		code.ErrInvalidCredential:     "invalid credential",
		code.ErrCredentialNotUsable:   "credential is not usable",
		code.ErrAuthenticationFailed:  "authentication failed",
		code.ErrOTPInvalid:            "OTP is invalid or expired",
		code.ErrStateMismatch:         "OAuth state mismatch",
		code.ErrIDPExchangeFailed:     "failed to exchange code with identity provider",
		code.ErrNoBinding:             "no login identity binding found",
		code.ErrOTPSendTooFrequent:    "OTP send too frequent",
		code.ErrUnsupportedAuthMethod: "unsupported authentication method",
		code.ErrPayloadInvalid:        "authentication payload is invalid",
		code.ErrProofBuildFailed:      "failed to build authentication proof",
		code.ErrUserBlocked:           "user is blocked",
		code.ErrUserInactive:          "user is inactive",
	}

	for codeValue, want := range tests {
		codeValue, want := codeValue, want
		t.Run(want, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, Message(codeValue))
		})
	}
}
