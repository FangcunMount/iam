package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type onboardingStrategy interface {
	Scenario() OnboardingScenario
	BuildPlan(req OnboardingRequest) (OnboardingPlan, error)
	Normalize(ctx context.Context, deps onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error)
}

type onboardingStrategyDeps struct {
	wechatIdentityResolver *wechatIdentityResolver
}

type onboardingStrategyRegistry struct {
	byScenario map[OnboardingScenario]onboardingStrategy
}

var defaultStrategies = newOnboardingStrategyRegistry(
	passwordCredentialStrategy{baseStrategy: newBaseStrategy(OnboardOperaPassword)},
	wechatMiniStrategy{baseStrategy: newBaseStrategy(OnboardWechatMini, withUserRepair())},
	wecomStrategy{baseStrategy: newBaseStrategy(OnboardWecom)},
	passwordCredentialStrategy{baseStrategy: newBaseStrategy(OnboardMockConsumerPassword)},
	phoneOTPStrategy{},
)

func newOnboardingStrategyRegistry(strategies ...onboardingStrategy) *onboardingStrategyRegistry {
	registry := &onboardingStrategyRegistry{
		byScenario: make(map[OnboardingScenario]onboardingStrategy, len(strategies)),
	}
	for _, strategy := range strategies {
		registry.byScenario[strategy.Scenario()] = strategy
	}
	return registry
}

func (r *onboardingStrategyRegistry) Select(req OnboardingRequest) (onboardingStrategy, error) {
	strategy, ok := r.byScenario[req.Scenario]
	if !ok {
		return nil, perrors.WithCode(
			code.ErrInvalidArgument,
			"unsupported onboarding scenario: %s",
			req.Scenario,
		)
	}
	return strategy, nil
}

type baseStrategy struct {
	scenario              OnboardingScenario
	allowUserRepair       bool
	allowCredentialRotate bool
}

type baseStrategyOption func(*baseStrategy)

func newBaseStrategy(scenario OnboardingScenario, opts ...baseStrategyOption) baseStrategy {
	strategy := baseStrategy{
		scenario: scenario,
	}
	for _, opt := range opts {
		opt(&strategy)
	}
	return strategy
}

func withUserRepair() baseStrategyOption {
	return func(strategy *baseStrategy) {
		strategy.allowUserRepair = true
	}
}

func (s baseStrategy) Scenario() OnboardingScenario {
	return s.scenario
}

func (s baseStrategy) BuildPlan(req OnboardingRequest) (OnboardingPlan, error) {
	return OnboardingPlan{
		Scenario:              s.scenario,
		NeedUser:              true,
		NeedLoginIdentity:     true,
		NeedCredential:        shouldCreateCredentialForScenario(s.scenario),
		AllowExistingUser:     true,
		AllowUserRepair:       s.allowUserRepair,
		AllowCredentialRotate: s.allowCredentialRotate,
	}, nil
}

func shouldCreateCredentialForScenario(scenario OnboardingScenario) bool {
	switch scenario {
	case OnboardOperaPassword, OnboardMockConsumerPassword:
		return true
	default:
		return false
	}
}

func (s baseStrategy) Normalize(_ context.Context, _ onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error) {
	return req, nil
}

type passwordCredentialStrategy struct {
	baseStrategy
}

type wechatMiniStrategy struct {
	baseStrategy
}

func (s wechatMiniStrategy) Normalize(ctx context.Context, deps onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error) {
	if deps.wechatIdentityResolver == nil {
		return req, nil
	}
	identity, err := deps.wechatIdentityResolver.ResolveMiniProgram(ctx, req)
	if err != nil {
		return req, err
	}
	return prepareWechatIdentity(req, identity), nil
}

type wecomStrategy struct {
	baseStrategy
}

type phoneOTPStrategy struct{}

func (s phoneOTPStrategy) Scenario() OnboardingScenario {
	return OnboardPhoneOTP
}

func (s phoneOTPStrategy) BuildPlan(req OnboardingRequest) (OnboardingPlan, error) {
	base := newBaseStrategy(OnboardPhoneOTP)
	return base.BuildPlan(req)
}

func (s phoneOTPStrategy) Normalize(_ context.Context, _ onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error) {
	return req, nil
}
