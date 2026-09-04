package jwt

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// tenantDomainFromClaims 将认证声明映射为 JWT tenant_id claim。
// 它负责将认证声明映射为 JWT tenant_id claim。
func tenantDomainFromClaims(claims map[string]any, realm string) string {
	// 如果认证声明中包含 tenant_domain，则返回 tenant_domain
	if domain := stringClaim(claims, "tenant_domain"); domain != "" {
		return domain
	}
	// 如果认证域不为空，则返回认证域
	if realm = strings.TrimSpace(realm); realm != "" {
		return realm
	}
	// 如果认证域为空，则返回默认租户域
	return tenant.DefaultID
}

// stringClaim 从认证声明中获取字符串值。
// 它负责从认证声明中获取字符串值。
func stringClaim(claims map[string]any, key string) string {
	// 如果认证声明为空，则返回空字符串
	if len(claims) == 0 {
		return ""
	}
	// 获取认证声明的值
	value, ok := claims[key]
	// 如果认证声明的值为空，则返回空字符串
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
