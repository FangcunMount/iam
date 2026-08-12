package method

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// PasswordPayload 是密码登录 payload。
type PasswordPayload struct {
	Username string
	Password string
}

func (PasswordPayload) loginMethodPayload() {}

// passwordMethod 密码登录方式
type passwordMethod struct{}

// NewPasswordMethod 创建密码登录方式。
func NewPasswordMethod() LoginMethod {
	return passwordMethod{}
}

// Method 返回方法
func (passwordMethod) Method() AuthMethod {
	return AuthMethodPassword
}

// CredentialKind 返回认证证明类型
func (passwordMethod) CredentialKind() CredentialKind {
	return CredentialKindPassword
}

// BuildPayload 构建 payload
func (passwordMethod) BuildPayload(cmd LoginRequest) (Payload, error) {
	payload, ok := cmd.Payload.(PasswordPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "invalid password payload")
	}
	if payload.Username == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "username is required for password authentication")
	}
	if payload.Password == "" {
		return nil, perrors.WithCode(code.ErrPayloadInvalid, "password is required for password authentication")
	}
	return payload, nil
}
