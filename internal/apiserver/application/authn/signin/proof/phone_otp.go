package proof

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// phoneOTPBuilder 手机号验证码登录方式构造器
type phoneOTPBuilder struct{}

// NewPhoneOTPBuilder 创建手机号验证码登录方式构造器
func NewPhoneOTPBuilder() Builder {
	return phoneOTPBuilder{}
}

// CredentialKind 返回认证证明类型
func (phoneOTPBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindPhoneOTP
}

// Build 构建手机号验证码登录方式
func (phoneOTPBuilder) Build(_ context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	// 验证手机号验证码登录方式凭证是否有效
	phonePayload, ok := payload.(method.PhoneOTPPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "invalid phone otp payload")
	}

	// 构建手机号验证码登录方式凭证
	return authentication.NewPhoneOTPCredential(authentication.PhoneOTPProofSpec{
		TenantID:  common.TenantID,
		RemoteIP:  common.RemoteIP,
		UserAgent: common.UserAgent,
		PhoneE164: phonePayload.PhoneE164,
		OTP:       phonePayload.OTP,
	})
}
