package proof

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// phoneOTPBuilder 手机号验证码登录方式构造器
type phoneOTPBuilder struct{}

// NewPhoneOTPBuilder 创建手机号验证码登录方式构造器
func NewPhoneOTPBuilder() Builder {
	return phoneOTPBuilder{}
}

// CredentialKind 返回凭据类型
func (phoneOTPBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindPhoneOTP
}

// Build 构建手机号验证码登录方式
func (phoneOTPBuilder) Build(_ context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	phonePayload, ok := payload.(method.PhoneOTPPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid phone otp payload")
	}
	return authentication.NewPhoneOTPCredential(authentication.PhoneOTPProofSpec{
		TenantID:  common.TenantID,
		RemoteIP:  common.RemoteIP,
		UserAgent: common.UserAgent,
		PhoneE164: phonePayload.PhoneE164,
		OTP:       phonePayload.OTP,
	})
}
