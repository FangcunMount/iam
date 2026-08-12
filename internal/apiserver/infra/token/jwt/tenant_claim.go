package jwt

import (
	"strconv"
	"strings"

	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// parseTenantIDClaim 解析 JWT tenant_id：新 token 为授权域 string；历史 token 可能为数值 org 占位。
func parseTenantIDClaim(raw string) (domain string, legacyNumericOrg uint64) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return tenant.DefaultID, 0
	}
	if n, err := strconv.ParseUint(raw, 10, 64); err == nil {
		return tenant.DefaultID, n
	}
	return raw, 0
}
