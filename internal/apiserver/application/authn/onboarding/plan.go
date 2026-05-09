package onboarding

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	accountDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
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

// OnboardingPlan 把场景对应的账号、凭据和幂等策略显式化。
type OnboardingPlan struct {
	Scenario       OnboardingScenario
	AccountType    accountDomain.AccountType
	CredentialType CredentialType

	NeedUser       bool
	NeedAccount    bool
	NeedCredential bool

	AllowExistingUser     bool
	AllowUserRepair       bool
	AllowCredentialReuse  bool
	AllowCredentialRotate bool
}

// BuildPlan 根据场景生成账号开通计划。
//
// 为了兼容现有调用方，Scenario 为空时会从 AccountType + CredentialType 推导；
// 一旦调用方开始传 Scenario，AccountType / CredentialType 必须与场景匹配。
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
	if err := validateRequestedTypes(req, plan); err != nil {
		return OnboardingPlan{}, err
	}
	if err := validateScopedTenant(req, plan); err != nil {
		return OnboardingPlan{}, err
	}
	return plan, nil
}

func validateRequestedTypes(req OnboardingRequest, plan OnboardingPlan) error {
	if req.AccountType != "" && req.AccountType != plan.AccountType {
		return perrors.WithCode(
			code.ErrInvalidArgument,
			"account_type %s does not match onboarding scenario %s",
			req.AccountType,
			plan.Scenario,
		)
	}
	if req.CredentialType != "" && req.CredentialType != plan.CredentialType {
		return perrors.WithCode(
			code.ErrInvalidArgument,
			"credential_type %s does not match onboarding scenario %s",
			req.CredentialType,
			plan.Scenario,
		)
	}
	return nil
}

func validateScopedTenant(req OnboardingRequest, plan OnboardingPlan) error {
	if plan.AccountType == accountDomain.TypeOpera && req.ScopedTenantID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is required for opera account")
	}
	if plan.AccountType != accountDomain.TypeOpera && !req.ScopedTenantID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is only valid for opera account")
	}
	return nil
}
