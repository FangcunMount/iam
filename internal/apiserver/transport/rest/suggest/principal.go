package suggest

import (
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
	return domainsuggest.OperatingPrincipal{
		OperatorID:   int64(uid),
		TenantID:     tenantNumericID(dom),
		TenantDomain: dom,
	}, true
}

func tenantNumericID(domain string) int64 {
	if domain == tenant.DefaultID || domain == "" {
		return int64(tenant.DefaultTenantID)
	}
	// 非默认租户且无映射时返回 0，由后续权限维度的 Org/Profile/负责人兜底；
	// 单租户部署应使用 fangcun 或显式在 Loader SQL 中写入 tenant_id。
	return 0
}
