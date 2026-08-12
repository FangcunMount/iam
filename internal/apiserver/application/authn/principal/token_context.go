package principal

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// EnsureTokenContext 为签发 JWT 准备授权域 claim；IAM 不写入默认 org_id。
func EnsureTokenContext(p *authentication.Principal) {
	if p == nil {
		return
	}
	if p.Claims == nil {
		p.Claims = make(map[string]any)
	}
	if _, ok := p.Claims["tenant_domain"]; !ok {
		p.Claims["tenant_domain"] = tenant.DefaultID
	}
}
