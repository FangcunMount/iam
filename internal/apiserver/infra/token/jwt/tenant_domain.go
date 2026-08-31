package jwt

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// tenantDomainFromClaims 将认证声明映射为 JWT tenant_id claim。
func tenantDomainFromClaims(claims map[string]any, realm string) string {
	if domain := stringClaim(claims, "tenant_domain"); domain != "" {
		return domain
	}
	if realm = strings.TrimSpace(realm); realm != "" {
		return realm
	}
	return tenant.DefaultID
}

func stringClaim(claims map[string]any, key string) string {
	if len(claims) == 0 {
		return ""
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
