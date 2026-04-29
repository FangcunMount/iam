package login

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

func buildPasswordProof(payload MethodPayload) (authentication.AuthCredential, error) {
	passwordPayload, ok := payload.(PasswordPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid password payload")
	}
	return authentication.NewPasswordCredential(authentication.PasswordProofSpec{
		TenantID:  passwordPayload.TenantID,
		RemoteIP:  passwordPayload.RemoteIP,
		UserAgent: passwordPayload.UserAgent,
		Username:  passwordPayload.Username,
		Password:  passwordPayload.Password,
	})
}

func buildPhoneOTPProof(payload MethodPayload) (authentication.AuthCredential, error) {
	phonePayload, ok := payload.(PhoneOTPPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid phone otp payload")
	}
	return authentication.NewPhoneOTPCredential(authentication.PhoneOTPProofSpec{
		TenantID:  phonePayload.TenantID,
		RemoteIP:  phonePayload.RemoteIP,
		UserAgent: phonePayload.UserAgent,
		PhoneE164: phonePayload.PhoneE164,
		OTP:       phonePayload.OTP,
	})
}

func buildWecomProof(payload MethodPayload) (authentication.AuthCredential, error) {
	wecomPayload, ok := payload.(WecomPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid wecom payload")
	}
	return authentication.NewWecomCredential(authentication.WecomProofSpec{
		TenantID: wecomPayload.TenantID,
		CorpID:   wecomPayload.CorpID,
		Code:     wecomPayload.Code,
	})
}
