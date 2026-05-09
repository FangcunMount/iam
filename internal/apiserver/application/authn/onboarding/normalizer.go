package onboarding

import (
	"context"
	"strings"
)

// NormalizedOnboardingRequest 是进入事务内流程前的稳定输入。
type NormalizedOnboardingRequest struct {
	OnboardingRequest
	Plan     OnboardingPlan
	strategy onboardingStrategy
}

type requestNormalizer struct {
	wechatIdentityResolver *wechatIdentityResolver
}

func newRequestNormalizer(wechatIdentityResolver *wechatIdentityResolver) *requestNormalizer {
	return &requestNormalizer{wechatIdentityResolver: wechatIdentityResolver}
}

// Normalize 在数据库事务外完成场景计划、字段归一化和第三方身份解析。
func (n *requestNormalizer) Normalize(ctx context.Context, req OnboardingRequest) (*NormalizedOnboardingRequest, error) {
	prepared := trimRequest(req)

	strategy, err := defaultStrategies.Select(prepared)
	if err != nil {
		return nil, err
	}
	plan, err := buildPlanFromStrategy(strategy, prepared)
	if err != nil {
		return nil, err
	}
	prepared.Scenario = plan.Scenario

	prepared, err = strategy.Normalize(ctx, onboardingStrategyDeps{
		wechatIdentityResolver: n.wechatIdentityResolver,
	}, prepared)
	if err != nil {
		return nil, err
	}

	return &NormalizedOnboardingRequest{
		OnboardingRequest: prepared,
		Plan:              plan,
		strategy:          strategy,
	}, nil
}

func trimRequest(req OnboardingRequest) OnboardingRequest {
	req.Name = strings.TrimSpace(req.Name)
	req.OperaLoginID = strings.TrimSpace(req.OperaLoginID)
	req.WechatAppID = trimStringPtr(req.WechatAppID)
	req.WechatJsCode = trimStringPtr(req.WechatJsCode)
	req.WechatOpenID = trimStringPtr(req.WechatOpenID)
	req.WechatUnionID = trimStringPtr(req.WechatUnionID)
	req.WecomCorpID = trimStringPtr(req.WecomCorpID)
	req.WecomUserID = trimStringPtr(req.WecomUserID)
	return req
}

func trimStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	return &trimmed
}
