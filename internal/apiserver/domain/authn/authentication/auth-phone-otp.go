package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ====================== 认证凭据（认证所需的数据） ========================

// PhoneOTPCredential 认证凭据（手机号+验证码）
type PhoneOTPCredential struct {
	TenantID  meta.ID // 认证域
	RemoteIP  string  // 认证客户端IP
	UserAgent string  // 认证客户端UA
	PhoneE164 string  // 手机号
	OTP       string  // 验证码
}

// PhoneOTPProofSpec 手机号验证码认证凭据规格
type PhoneOTPProofSpec struct {
	TenantID  meta.ID // 认证域
	RemoteIP  string  // 认证客户端IP
	UserAgent string  // 认证客户端UA
	PhoneE164 string  // 手机号
	OTP       string  // 验证码
}

// CredentialKind 返回认证证明类型。
func (c *PhoneOTPCredential) CredentialKind() CredentialKind {
	return CredentialKindPhoneOTP
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
	credentialKind CredentialKind
	identityRepo   LoginIdentityRepository
	otpVerifier    OTPVerifier
}

// 实现认证策略接口
var _ AuthStrategy = (*PhoneOTPAuthStrategy)(nil)

const phoneOTPLoginScene = "login"

func NewPhoneOTPAuthStrategyWithLoginIdentity(
	identityRepo LoginIdentityRepository,
	otpVerifier OTPVerifier,
) *PhoneOTPAuthStrategy {
	return &PhoneOTPAuthStrategy{
		credentialKind: CredentialKindPhoneOTP,
		identityRepo:   identityRepo,
		otpVerifier:    otpVerifier,
	}
}

// Kind 返回认证策略类型
func (p *PhoneOTPAuthStrategy) Kind() CredentialKind {
	return p.credentialKind
}

// Authenticate 执行手机验证码认证
// 认证流程：
// 1. 验证并消费OTP（防止重放攻击）
// 2. 根据手机号查找凭据绑定
// 3. 检查 LoginIdentity 状态
// 4. 返回认证判决
func (p *PhoneOTPAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	otpCredential, ok := credential.(*PhoneOTPCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("phone otp strategy expects *PhoneOTPCredential, got %T", credential)
	}
	if !p.verifyLoginOTP(ctx, otpCredential) {
		return AuthDecision{
			OK:   false,
			Code: code.ErrOTPInvalid,
		}, nil
	}

	lookup, err := p.identityRepo.FindLoginIdentityByProviderKey(
		ctx,
		loginidentity.ProviderPhone,
		loginidentity.RealmGlobal,
		otpCredential.PhoneE164,
	)
	if err != nil {
		return AuthDecision{}, fmt.Errorf("failed to find phone login identity: %w", err)
	}
	if lookup == nil || lookup.LoginIdentityID.IsZero() {
		return AuthDecision{
			OK:   false,
			Code: code.ErrNoBinding,
		}, nil
	}

	statusFailure, err := loginIdentityStatusFailureDecision(ctx, p.identityRepo, lookup.LoginIdentityID)
	if err != nil {
		return AuthDecision{}, err
	}
	if statusFailure != nil {
		return *statusFailure, nil
	}

	return p.buildPhoneOTPSuccessDecision(
		ctx,
		otpCredential,
		lookup.LoginIdentityID,
		lookup.UserID,
		meta.ZeroID,
	), nil
}

// verifyLoginOTP 验证OTP并标记为已使用
func (p *PhoneOTPAuthStrategy) verifyLoginOTP(ctx context.Context, credential *PhoneOTPCredential) bool {
	return p.otpVerifier.VerifyAndConsume(ctx, credential.PhoneE164, phoneOTPLoginScene, credential.OTP)
}

// buildPhoneOTPSuccessDecision 认证成功，构造Principal
func (p *PhoneOTPAuthStrategy) buildPhoneOTPSuccessDecision(
	ctx context.Context,
	credential *PhoneOTPCredential,
	loginIdentityID meta.ID,
	userID meta.ID,
	credentialID meta.ID,
) AuthDecision {
	principal := &Principal{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		TenantID:        credential.TenantID,
		AuthMethod:      "phone_otp",
		Realm:           loginidentity.RealmGlobal,
		AMR:             []string{string(AMROTP)},
		Claims: map[string]any{
			"phone_number":      credential.PhoneE164,
			"login_identity_id": loginIdentityID.String(),
			"auth_method":       "phone_otp",
			"realm":             loginidentity.RealmGlobal,
			"auth_time":         ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:              true,
		Principal:       principal,
		LoginIdentityID: loginIdentityID,
		CredentialID:    credentialID,
	}
}
