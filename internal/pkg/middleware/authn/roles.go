package authn

import "strings"

const (
	PlatformAdminRoleSuperAdmin    = "super_admin"
	PlatformAdminRolePlatformAdmin = "platform:admin"
	PlatformAdminRoleIAMAdmin      = "iam:admin"
)

// NormalizeRoleName 将 Casbin `role:<name>` 规整为前端 / 业务层使用的角色名。
func NormalizeRoleName(role string) string {
	role = strings.TrimSpace(role)
	if after, ok := strings.CutPrefix(role, "role:"); ok {
		return after
	}
	return role
}

// IsPlatformAdminRole 判断角色是否属于平台控制面管理员。
func IsPlatformAdminRole(role string) bool {
	switch NormalizeRoleName(role) {
	case PlatformAdminRoleSuperAdmin, PlatformAdminRolePlatformAdmin, PlatformAdminRoleIAMAdmin:
		return true
	default:
		return false
	}
}
