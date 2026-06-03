package method

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// WechatScanPayload 是微信扫码登录 payload。
type WechatScanPayload struct {
	AppID string
	Code  string
	State string
}

func (WechatScanPayload) loginMethodPayload() {}

// wechatScanMethod 微信扫码登录方式
type wechatScanMethod struct{}

// NewWechatScanMethod 创建微信扫码登录方式。
func NewWechatScanMethod() LoginMethod {
	return wechatScanMethod{}
}

// Method 返回方法
func (wechatScanMethod) Method() AuthMethod {
	return AuthMethodWechatScan
}

// CredentialKind 返回认证证明类型
func (wechatScanMethod) CredentialKind() CredentialKind {
	return CredentialKindWechatScan
}

// BuildPayload 构建 payload
func (wechatScanMethod) BuildPayload(cmd LoginRequest) (Payload, error) {
	payload, ok := cmd.Payload.(WechatScanPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "invalid wechat scan payload")
	}
	if payload.AppID == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "app_id is required for wechat scan authentication")
	}
	if payload.Code == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "code is required for wechat scan authentication")
	}
	if payload.State == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "state is required for wechat scan authentication")
	}
	return payload, nil
}
