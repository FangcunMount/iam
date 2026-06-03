package method

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// WechatMiniPayload 是微信小程序登录 payload。
type WechatMiniPayload struct {
	AppID  string
	JSCode string
}

func (WechatMiniPayload) loginMethodPayload() {}

// wechatMethod 微信小程序登录方式
type wechatMiniMethod struct{}

// NewWechatMiniMethod 创建微信小程序登录方式。
func NewWechatMiniMethod() LoginMethod {
	return wechatMiniMethod{}
}

// Method 返回方法
func (wechatMiniMethod) Method() AuthMethod {
	return AuthMethodWechatMini
}

// CredentialKind 返回认证证明类型
func (wechatMiniMethod) CredentialKind() CredentialKind {
	return CredentialKindWechatMinip
}

// BuildPayload 构建 payload
func (wechatMiniMethod) BuildPayload(cmd LoginRequest) (Payload, error) {
	payload, ok := cmd.Payload.(WechatMiniPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "invalid wechat payload")
	}
	if payload.AppID == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "app_id is required for wechat authentication")
	}
	if payload.JSCode == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "code is required for wechat authentication")
	}
	return payload, nil
}
