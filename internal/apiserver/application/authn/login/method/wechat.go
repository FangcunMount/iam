package method

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// WechatPayload 是微信小程序登录 payload。
type WechatPayload struct {
	AppID  string
	JSCode string
}

func (WechatPayload) loginMethodPayload() {}

// wechatMethod 微信小程序登录方式
type wechatMethod struct{}

// NewWechatMethod 创建微信小程序登录方式。
func NewWechatMethod() LoginMethod {
	return wechatMethod{}
}

// Method 返回方法
func (wechatMethod) Method() AuthMethod {
	return AuthMethodWechat
}

// CredentialKind 返回凭据类型
func (wechatMethod) CredentialKind() CredentialKind {
	return CredentialKindWechatMinip
}

// BuildPayload 构建 payload
func (wechatMethod) BuildPayload(cmd LoginRequest) (Payload, error) {
	payload, ok := cmd.Payload.(WechatPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid wechat payload")
	}
	if payload.AppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "app_id is required for wechat authentication")
	}
	if payload.JSCode == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "code is required for wechat authentication")
	}
	return payload, nil
}
