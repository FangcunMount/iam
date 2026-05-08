package login

import (
	"encoding/json"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/compatibility"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
)

// =============================== Public Auth Methods ===============================

// PublicAuthMethods 返回公开认证方法。
func PublicAuthMethods() []AuthMethod {
	return method.PublicAuthMethods()
}

// IsPublicAuthMethod 判断是否是公开认证方法。
func IsPublicAuthMethod(raw string) bool {
	return method.IsPublicAuthMethod(raw)
}

// BuildExplicitLoginRequest 构建显式登录请求。
func BuildExplicitLoginRequest(authMethod string, payload json.RawMessage) (LoginRequest, error) {
	return compatibility.BuildExplicitWireLoginRequest(authMethod, payload)
}
