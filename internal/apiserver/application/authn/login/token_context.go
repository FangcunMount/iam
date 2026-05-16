package login

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// ensurePrincipalTokenContext 为签发 JWT 准备授权域 claim；IAM 不写入默认 org_id。
func ensurePrincipalTokenContext(principal *authentication.Principal) {
	if principal == nil {
		return
	}
	if principal.Claims == nil {
		principal.Claims = make(map[string]any)
	}
	if _, ok := principal.Claims["tenant_domain"]; !ok {
		principal.Claims["tenant_domain"] = tenant.DefaultID
	}
}
