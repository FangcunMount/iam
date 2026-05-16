package verifier

import (
	"strconv"
	"strings"
)

// AuthorizationDomain 返回 IAM 授权域（Casbin domain）。
func (c *TokenClaims) AuthorizationDomain() string {
	if c == nil {
		return ""
	}
	if domain := strings.TrimSpace(c.TenantDomain); domain != "" {
		return domain
	}
	return strings.TrimSpace(c.TenantID)
}

// BusinessOrgID 读取 JWT 透传的业务组织 ID；无 org_id 时 ok=false。
func (c *TokenClaims) BusinessOrgID() (uint64, bool) {
	if c == nil {
		return 0, false
	}
	raw := strings.TrimSpace(c.OrgID)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0, false
	}
	return n, true
}
