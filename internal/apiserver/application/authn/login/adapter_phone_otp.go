package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type phoneOTPAdapter struct{}

func newPhoneOTPAdapter() phoneOTPAdapter {
	return phoneOTPAdapter{}
}

func (phoneOTPAdapter) Kind() SignInKind {
	return SignInKind(credDomain.CredPhoneOTP)
}

func (phoneOTPAdapter) AuthType() AuthType {
	return AuthTypePhoneOTP
}

func (phoneOTPAdapter) TryLegacy(req SignInCommand, common methodPayloadCommon) (MethodPayload, bool) {
	if req.PhoneE164 == nil || req.OTPCode == nil {
		return nil, false
	}
	return PhoneOTPPayload{
		methodPayloadCommon: common,
		PhoneE164:           *req.PhoneE164,
		OTP:                 *req.OTPCode,
	}, true
}

func (phoneOTPAdapter) BuildExplicit(req SignInCommand, common methodPayloadCommon) (MethodPayload, error) {
	if req.PhoneE164 == nil || *req.PhoneE164 == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "phone is required for phone otp authentication")
	}
	if req.OTPCode == nil || *req.OTPCode == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "otp_code is required for phone otp authentication")
	}
	return PhoneOTPPayload{
		methodPayloadCommon: common,
		PhoneE164:           *req.PhoneE164,
		OTP:                 *req.OTPCode,
	}, nil
}

func (phoneOTPAdapter) PrepareProof(_ context.Context, payload MethodPayload) (authentication.AuthCredential, error) {
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
