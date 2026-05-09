package onboarding

import (
	"context"
)

// ============= 用例入口（Driving Port）=============

// LoginIdentityOnboarder 负责登录身份开通用例。
//
// 对外调用方只依赖这个入口；请求归一化、用户解析、登录身份确保、凭据确保都属于用例内部流程。
type LoginIdentityOnboarder interface {
	// Onboard 完成：1) 归一化请求 2) 解析或创建 User 3) 确保 LoginIdentity 存在 4) 按需确保 Credential 存在。
	Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error)
}
