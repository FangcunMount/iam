package verifier

import (
	"strconv"
	"strings"

	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// normalizeTenantClaim 将 JWT tenant_id 归一化为 IAM 授权域；历史数值 token 映射为默认域，不推导 org。
func normalizeTenantClaim(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return tenant.DefaultID
	}
	if _, err := strconv.ParseUint(raw, 10, 64); err == nil {
		return tenant.DefaultID
	}
	return raw
}

// applyTenantAndOrg 写入授权域与业务 org_id；TenantID 与 TenantDomain 同步以保持兼容。
func applyTenantAndOrg(claims *TokenClaims, tenantRaw, orgRaw string) {
	if claims == nil {
		return
	}
	domain := normalizeTenantClaim(tenantRaw)
	claims.TenantDomain = domain
	claims.TenantID = domain
	claims.OrgID = strings.TrimSpace(orgRaw)
}
