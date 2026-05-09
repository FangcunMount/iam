package onboarding

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// OnboardingScenario 表示账号开通的业务场景。
type OnboardingScenario string

const (
	OnboardOperaPassword        OnboardingScenario = "opera_password"
	OnboardWechatMini           OnboardingScenario = "wechat_mini"
	OnboardWecom                OnboardingScenario = "wecom"
	OnboardPhoneOTP             OnboardingScenario = "phone_otp"
	OnboardMockConsumerPassword OnboardingScenario = "mock_consumer_password"
)

// OnboardingPlan 把场景对应的登录身份、凭据和幂等策略显式化。
type OnboardingPlan struct {
	Scenario OnboardingScenario

	NeedUser          bool
	NeedLoginIdentity bool
	NeedCredential    bool

	AllowExistingUser     bool
	AllowUserRepair       bool
	AllowCredentialRotate bool
}

// BuildPlan 根据场景生成登录身份开通计划。
func BuildPlan(req OnboardingRequest) (OnboardingPlan, error) {
	strategy, err := defaultStrategies.Select(req)
	if err != nil {
		return OnboardingPlan{}, err
	}
	return buildPlanFromStrategy(strategy, req)
}

func buildPlanFromStrategy(strategy onboardingStrategy, req OnboardingRequest) (OnboardingPlan, error) {
	plan, err := strategy.BuildPlan(req)
	if err != nil {
		return OnboardingPlan{}, err
	}
	if err := validateScopedTenant(req, plan); err != nil {
		return OnboardingPlan{}, err
	}
	return plan, nil
}

func validateScopedTenant(req OnboardingRequest, plan OnboardingPlan) error {
	if plan.Scenario == OnboardOperaPassword && req.ScopedTenantID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is required for opera login identity")
	}
	if plan.Scenario != OnboardOperaPassword && !req.ScopedTenantID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is only valid for opera login identity")
	}
	return nil
}
