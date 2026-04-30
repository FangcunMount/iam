package login

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type MethodKind string

const (
	MethodPassword    MethodKind = "password"
	MethodPhoneOTP    MethodKind = "phone_otp"
	MethodWechatMini  MethodKind = "oauth_wx_minip"
	MethodWecom       MethodKind = "oauth_wecom"
	MethodBearerToken MethodKind = "jwt_token"
)

// SelectedMethod 是应用层完成方法选择后的认证输入。
type SelectedMethod struct {
	Method  MethodKind
	Payload MethodPayload
}

func (m SelectedMethod) TenantID() meta.ID {
	if m.Payload == nil {
		return meta.ZeroID
	}
	return m.Payload.commonPayload().TenantID
}

// ScenarioSelector 将登录请求转换为领域认证场景和输入。
type ScenarioSelector interface {
	Select(ctx context.Context, req LoginRequest) (SelectedMethod, error)
}

type defaultScenarioSelector struct {
	legacy   legacyScenarioSelector
	explicit explicitScenarioSelector
}

func newDefaultScenarioSelector() ScenarioSelector {
	return defaultScenarioSelector{}
}

func (s defaultScenarioSelector) Select(ctx context.Context, req LoginRequest) (SelectedMethod, error) {
	if req.SelectionMode == ScenarioSelectionExplicit {
		return s.explicit.Select(ctx, req)
	}
	return s.legacy.Select(ctx, req)
}
