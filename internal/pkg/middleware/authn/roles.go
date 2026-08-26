package authn

import "strings"

// NormalizeRoleName 将历史兼容前缀 `role:<name>` 规整为前端 / 业务层使用的角色名。
func NormalizeRoleName(role string) string {
	role = strings.TrimSpace(role)
	if after, ok := strings.CutPrefix(role, "role:"); ok {
		return after
	}
	return role
}
