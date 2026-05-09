package onboarding

import (
	"context"
)

// ============= 用例入口（Driving Port）=============

// AccountOnboarder 负责账号开通用例。
//
// 对外调用方只依赖这个入口；请求归一化、用户解析、账号确保、凭据确保都属于用例内部流程。
type AccountOnboarder interface {
	// Onboard 完成：1) 归一化请求 2) 解析或创建 User 3) 确保 Account 存在 4) 确保 Credential 存在 5) 返回账号开通结果。
	Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error)
}
