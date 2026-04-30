package login

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type AuthFailureTranslator struct{}

func (AuthFailureTranslator) Translate(errCode authentication.ErrCode) error {
	switch errCode {
	case authentication.ErrInvalidCredential:
		return perrors.WithCode(code.ErrPasswordIncorrect, "invalid credentials")
	case authentication.ErrOTPMissingOrExpiry:
		return perrors.WithCode(code.ErrOTPInvalid, "OTP is invalid or expired")
	case authentication.ErrNoBinding:
		return perrors.WithCode(code.ErrNoBinding, "no account binding found")
	case authentication.ErrLocked:
		return perrors.WithCode(code.ErrCredentialLocked, "account is locked")
	case authentication.ErrDisabled:
		return perrors.WithCode(code.ErrCredentialDisabled, "account is disabled")
	case authentication.ErrIDPExchangeFailed:
		return perrors.WithCode(code.ErrIDPExchangeFailed, "failed to exchange with identity provider")
	case authentication.ErrStateMismatch:
		return perrors.WithCode(code.ErrStateMismatch, "state parameter mismatch")
	default:
		return perrors.WithCode(code.ErrAuthenticationFailed, "authentication failed")
	}
}
