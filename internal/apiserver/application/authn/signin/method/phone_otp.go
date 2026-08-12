package method

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// PhoneOTPPayload 是手机号验证码登录 payload。
type PhoneOTPPayload struct {
	PhoneE164 string
	OTP       string
}

func (PhoneOTPPayload) loginMethodPayload() {}

// phoneOTPMethod 手机号验证码登录方式
type phoneOTPMethod struct{}

// NewPhoneOTPMethod 创建手机号验证码登录方式。
func NewPhoneOTPMethod() LoginMethod {
	return phoneOTPMethod{}
}

// Method 返回方法
func (phoneOTPMethod) Method() AuthMethod {
	return AuthMethodPhoneOTP
}

// CredentialKind 返回认证证明类型
func (phoneOTPMethod) CredentialKind() CredentialKind {
	return CredentialKindPhoneOTP
}

// BuildPayload 构建 payload
func (phoneOTPMethod) BuildPayload(cmd LoginRequest) (Payload, error) {
	payload, ok := cmd.Payload.(PhoneOTPPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "invalid phone otp payload")
	}
	if payload.PhoneE164 == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "phone is required for phone otp authentication")
	}
	if payload.OTP == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "otp_code is required for phone otp authentication")
	}
	return payload, nil
}
