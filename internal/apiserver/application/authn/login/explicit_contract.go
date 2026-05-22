package login

import (
	"encoding/json"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/compatibility"
)

// BuildExplicitLoginRequest 将 REST/gRPC 显式 wire payload 转成 LoginRequest。
func BuildExplicitLoginRequest(authMethod string, payload json.RawMessage) (LoginRequest, error) {
	return compatibility.BuildExplicitWireLoginRequest(authMethod, payload)
}
