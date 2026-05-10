package login

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

func authFailureError(codeValue int) error {
	if codeValue == 0 {
		codeValue = code.ErrAuthenticationFailed
	}
	return perrors.WithCode(codeValue, "%s", authFailureMessage(codeValue))
}

func authFailureMessage(codeValue int) string {
	switch codeValue {
	case code.ErrUnauthenticated:
		return "authentication failed"
	case code.ErrInvalidCredentials:
		return "invalid credentials"
	case code.ErrTokenInvalid:
		return "token invalid"
	case code.ErrSignatureInvalid:
		return "signature invalid"
	case code.ErrExpired:
		return "token expired"
	case code.ErrUserNotRegistered:
		return "user not registered"
	case code.ErrLoginIdentityDisabled:
		return "login identity is disabled"
	case code.ErrCredentialLocked:
		return "credential is locked"
	case code.ErrCredentialExpired:
		return "credential has expired"
	case code.ErrCredentialDisabled:
		return "credential is disabled"
	case code.ErrInvalidCredential:
		return "invalid credential"
	case code.ErrCredentialNotUsable:
		return "credential is not usable"
	case code.ErrAuthenticationFailed:
		return "authentication failed"
	case code.ErrOTPInvalid:
		return "OTP is invalid or expired"
	case code.ErrStateMismatch:
		return "OAuth state mismatch"
	case code.ErrIDPExchangeFailed:
		return "failed to exchange code with identity provider"
	case code.ErrNoBinding:
		return "no login identity binding found"
	case code.ErrOTPSendTooFrequent:
		return "OTP send too frequent"
	case code.ErrUnsupportedAuthMethod:
		return "unsupported authentication method"
	case code.ErrPayloadInvalid:
		return "authentication payload is invalid"
	case code.ErrProofBuildFailed:
		return "failed to build authentication proof"
	case code.ErrUserBlocked:
		return "user is blocked"
	case code.ErrUserInactive:
		return "user is inactive"
	default:
		return "authentication failed"
	}
}
