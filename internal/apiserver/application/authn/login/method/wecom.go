package method

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// WecomPayload 是企业微信登录 payload。
type WecomPayload struct {
	CorpID string
	Code   string
}

func (WecomPayload) loginMethodPayload() {}

// wecomMethod 企业微信登录方式
type wecomMethod struct{}

// NewWecomMethod 创建企业微信登录方式。
func NewWecomMethod() LoginMethod {
	return wecomMethod{}
}

// Method 返回方法
func (wecomMethod) Method() AuthMethod {
	return AuthMethodWecom
}

// CredentialKind 返回凭据类型
func (wecomMethod) CredentialKind() CredentialKind {
	return CredentialKindWecom
}

// BuildPayload 构建 payload
func (wecomMethod) BuildPayload(cmd LoginRequest) (Payload, error) {
	payload, ok := cmd.Payload.(WecomPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "invalid wecom payload")
	}
	if payload.CorpID == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "corp_id is required for wecom authentication")
	}
	if payload.Code == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "auth_code is required for wecom authentication")
	}
	return payload, nil
}
