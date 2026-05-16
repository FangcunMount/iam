package suggest

import (
	"strconv"

	"github.com/gin-gonic/gin"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// OperatingPrincipalFromGin 从 JWT 上下文提取 suggest 用身份快照。
func OperatingPrincipalFromGin(c *gin.Context) (domainsuggest.OperatingPrincipal, bool) {
	if c == nil {
		return domainsuggest.OperatingPrincipal{}, false
	}
	uid, ok := requestctx.UserID(c)
	if !ok || uid.IsZero() {
		return domainsuggest.OperatingPrincipal{}, false
	}
	dom := requestctx.TenantIDOrDefault(c)
	principal := domainsuggest.OperatingPrincipal{
		OperatorID:   int64(uid),
		TenantDomain: resolveAuthorizationDomain(dom),
	}
	if orgID, ok := requestctx.BusinessOrgID(c); ok {
		principal.OrgID = int64(orgID)
	}
	return principal, true
}

// resolveAuthorizationDomain 将 JWT 上下文中的 tenant 标识解析为授权域 string。
// 数值 tenant_id（历史误用为 org_id）映射回默认业务域 fangcun。
func resolveAuthorizationDomain(raw string) string {
	if raw == "" || raw == tenant.DefaultID || raw == tenant.PlatformID {
		if raw == "" {
			return tenant.DefaultID
		}
		return raw
	}
	if _, err := strconv.ParseUint(raw, 10, 64); err == nil {
		return tenant.DefaultID
	}
	return raw
}
