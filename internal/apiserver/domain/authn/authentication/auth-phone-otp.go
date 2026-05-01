package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ====================== 认证凭据（认证所需的数据） ========================

// PhoneOTPCredential 认证凭据（手机号+验证码）
type PhoneOTPCredential struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	PhoneE164 string
	OTP       string
}

type PhoneOTPProofSpec struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	PhoneE164 string
	OTP       string
}

// CredentialType 返回凭据类型。
func (c *PhoneOTPCredential) CredentialType() credDomain.CredentialType {
	return credDomain.CredPhoneOTP
}

// NewPhoneOTPCredential 构造手机号验证码认证凭据
func NewPhoneOTPCredential(spec PhoneOTPProofSpec) (AuthCredential, error) {
	if spec.PhoneE164 == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "phone number is required for phone otp authentication")
	}
	if spec.OTP == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "otp code is required for phone otp authentication")
	}

	return &PhoneOTPCredential{
		TenantID:  spec.TenantID,
		RemoteIP:  spec.RemoteIP,
		UserAgent: spec.UserAgent,
		PhoneE164: spec.PhoneE164,
		OTP:       spec.OTP,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// PhoneOTPAuthStrategy 手机短信验证码认证策略
type PhoneOTPAuthStrategy struct {
	credentialType credDomain.CredentialType
	credRepo       CredentialRepository
	accountRepo    AccountRepository
	otpVerifier    OTPVerifier
}

// 实现认证策略接口
var _ AuthStrategy = (*PhoneOTPAuthStrategy)(nil)

// NewPhoneOTPAuthStrategy 构造函数（注入依赖）
func NewPhoneOTPAuthStrategy(
	credRepo CredentialRepository,
	accountRepo AccountRepository,
	otpVerifier OTPVerifier,
) *PhoneOTPAuthStrategy {
	return &PhoneOTPAuthStrategy{
		credentialType: credDomain.CredPhoneOTP,
		credRepo:       credRepo,
		accountRepo:    accountRepo,
		otpVerifier:    otpVerifier,
	}
}

// Kind 返回认证策略类型
func (p *PhoneOTPAuthStrategy) Kind() credDomain.CredentialType {
	return p.credentialType
}

// Authenticate 执行手机验证码认证
// 认证流程：
// 1. 验证并消费OTP（防止重放攻击）
// 2. 根据手机号查找凭据绑定
// 3. 检查账户状态
// 4. 返回认证判决
func (p *PhoneOTPAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	otpCredential, ok := credential.(*PhoneOTPCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("phone otp strategy expects *PhoneOTPCredential, got %T", credential)
	}

	// Step 1: 验证OTP并标记为已使用
	const otpScene = "login" // OTP场景：登录
	if !p.otpVerifier.VerifyAndConsume(ctx, otpCredential.PhoneE164, otpScene, otpCredential.OTP) {
		// 业务失败：OTP无效或已过期
		return AuthDecision{
			OK:      false,
			ErrCode: ErrOTPMissingOrExpiry,
		}, nil
	}

	// Step 2: 根据手机号查找凭据绑定
	accountID, userID, credentialID, err := p.credRepo.FindPhoneOTPCredential(ctx, otpCredential.PhoneE164)
	if err != nil {
		return AuthDecision{}, fmt.Errorf("failed to find phone OTP credential: %w", err)
	}
	if credentialID.IsZero() {
		// 业务失败：手机号未绑定账户
		return AuthDecision{
			OK:      false,
			ErrCode: ErrNoBinding,
		}, nil
	}

	// Step 3: 检查账户状态
	enabled, locked, err := p.accountRepo.GetAccountStatus(ctx, accountID)
	if err != nil {
		return AuthDecision{}, fmt.Errorf("failed to get account status: %w", err)
	}
	if !enabled {
		return AuthDecision{
			OK:      false,
			ErrCode: ErrDisabled,
		}, nil
	}
	if locked {
		return AuthDecision{
			OK:      false,
			ErrCode: ErrLocked,
		}, nil
	}

	// Step 4: 认证成功，构造Principal
	principal := &Principal{
		AccountID: accountID,
		UserID:    userID,
		TenantID:  otpCredential.TenantID,
		AMR:       []string{string(AMROTP)},
		Claims: map[string]any{
			"phone_number": otpCredential.PhoneE164,
			"auth_time":    ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:           true,
		Principal:    principal,
		CredentialID: credentialID,
	}, nil
}
