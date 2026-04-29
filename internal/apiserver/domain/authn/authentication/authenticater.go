package authentication

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// AuthStrategy 认证策略（领域服务接口）
type AuthStrategy interface {
	Kind() Scenario
	Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error)
}

// Authenticater 认证器
type Authenticater struct {
	credRepo      CredentialRepository
	accountRepo   AccountRepository
	hasher        PasswordHasher
	otpVerifier   OTPVerifier
	idp           IdentityProvider
	tokenVerifier TokenVerifier
}

// NewAuthenticater 创建认证器
func NewAuthenticater(
	credRepo CredentialRepository,
	accountRepo AccountRepository,
	hasher PasswordHasher,
	otpVerifier OTPVerifier,
	idp IdentityProvider,
	tokenVerifier TokenVerifier,
) *Authenticater {
	return &Authenticater{
		credRepo:      credRepo,
		accountRepo:   accountRepo,
		hasher:        hasher,
		otpVerifier:   otpVerifier,
		idp:           idp,
		tokenVerifier: tokenVerifier,
	}
}

// Authenticate 认证
// 统一流程：
// 1. 根据场景构建领域凭据
// 2. 获取并创建认证策略
// 3. 执行认证
func (a *Authenticater) Authenticate(ctx context.Context, scenario Scenario, input AuthInput) (AuthDecision, error) {
	l := logger.L(ctx)

	l.Debugw("开始认证流程（域层）",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"tenant_id", input.TenantID,
		"amr", []string{string(scenario)},
		"claims", make(map[string]any),
	)

	// 根据场景构建领域凭据
	credential, err := a.buildCredential(scenario, input)
	if err != nil {
		l.Warnw("构建认证凭据失败",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"error", err.Error(),
		)
		return AuthDecision{}, err
	}

	l.Debugw("认证凭据构建完成",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"credential_type", scenario,
		"amr", []string{string(scenario)},
		"claims", make(map[string]any),
	)

	// 创建认证策略
	strategy := a.createStrategy(scenario)
	if strategy == nil {
		l.Errorw("不支持的认证场景",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
		)
		return AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication scenario: %s", scenario)
	}

	l.Debugw("认证策略已创建",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"strategy", scenario,
	)

	// 执行认证
	l.Debugw("开始执行认证策略",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
	)

	decision, err := strategy.Authenticate(ctx, credential)
	if err != nil {
		l.Errorw("认证策略执行出错",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"error", err.Error(),
		)
		return AuthDecision{}, err
	}

	if !decision.OK {
		l.Warnw("认证不通过（域层）",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"err_code", string(decision.ErrCode),
		)
		return decision, nil
	}

	if decision.Principal != nil && decision.Principal.TenantID.IsZero() && !input.TenantID.IsZero() {
		decision.Principal.TenantID = input.TenantID
	}

	l.Debugw("认证成功（域层）",
		"action", logger.ActionLogin,
		"scenario", string(scenario),
		"user_id", decision.Principal.UserID.String(),
		"account_id", decision.Principal.AccountID.String(),
		"tenant_id", decision.Principal.TenantID.String(),
	)

	return decision, nil
}

// BuildCredential 根据认证场景构建领域凭据
func (a *Authenticater) buildCredential(kind Scenario, input AuthInput) (AuthCredential, error) {
	builder, err := getCredentialBuilder(kind)
	if err != nil {
		return nil, err
	}

	credential, err := builder(input)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, perrors.WithCode(code.ErrMissingECParams, "credential builder returned nil for scenario: %s", kind)
	}
	return credential, nil
}

// CreateStrategy 根据场景创建认证策略
func (f *Authenticater) createStrategy(scenario Scenario) AuthStrategy {
	switch scenario {
	case AuthPassword:
		return NewPasswordAuthStrategy(f.credRepo, f.accountRepo, f.hasher)
	case AuthPhoneOTP:
		return NewPhoneOTPAuthStrategy(f.credRepo, f.accountRepo, f.otpVerifier)
	case AuthWxMinip:
		return NewOAuthWechatMinipAuthStrategy(f.credRepo, f.accountRepo, f.idp)
	case AuthWecom:
		return NewOAuthWeChatComAuthStrategy(f.credRepo, f.accountRepo, f.idp)
	case AuthBearerToken:
		return NewBearerTokenAuthStrategy(f.tokenVerifier)
	default:
		return nil
	}
}
