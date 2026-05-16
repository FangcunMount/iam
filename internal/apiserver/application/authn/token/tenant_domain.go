package token

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// TenantDomainFromClaims 从 Principal.Claims 解析 IAM 授权域；缺省为 tenant.DefaultID。
func TenantDomainFromClaims(claims map[string]any, realm string) string {
	if domain := stringClaim(claims, "tenant_domain"); domain != "" {
		return domain
	}
	realm = strings.TrimSpace(realm)
	if realm != "" {
		return realm
	}
	return tenant.DefaultID
}

func stringClaim(claims map[string]any, key string) string {
	if len(claims) == 0 {
		return ""
	}
	v, ok := claims[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
