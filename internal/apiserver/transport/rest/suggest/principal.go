package suggest

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// OperatingPrincipalFromGin 从 JWT 上下文提取 suggest 用身份快照。
func OperatingPrincipalFromGin(c *gin.Context) (visibility.Principal, bool) {
	if c == nil {
		return visibility.Principal{}, false
	}
	uid, ok := requestctx.UserID(c)
	if !ok || uid.IsZero() {
		return visibility.Principal{}, false
	}
	dom := requestctx.TenantIDOrDefault(c)
	principal := visibility.Principal{
		OperatorID:   int64(uid),
		TenantDomain: resolveAuthorizationDomain(dom),
	}
	if orgID, ok := requestctx.BusinessOrgID(c); ok {
		principal.OrgID = int64(orgID)
	}
	return principal, true
}

// resolveAuthorizationDomain 将 JWT 上下文中的 tenant 标识解析为授权域 string。
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
