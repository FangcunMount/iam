package role

import (
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Role 角色领域对象（聚合根）
type Role struct {
	ID          meta.ID
	Name        string // 角色名称
	DisplayName string // 显示名称
	TenantID    string // 租户ID
	Description string // 描述
}

// NewRole 创建新角色
func NewRole(name, displayName, tenantID string, opts ...RoleOption) Role {
	role := Role{
		Name:        name,
		DisplayName: displayName,
		TenantID:    tenantID,
	}
	for _, opt := range opts {
		opt(&role)
	}
	return role
}

// RoleOption 角色选项
type RoleOption func(*Role)

func WithID(id meta.ID) RoleOption           { return func(r *Role) { r.ID = id } }
func WithDescription(desc string) RoleOption { return func(r *Role) { r.Description = desc } }

// Key 返回 Casbin 中的角色标识
func (r *Role) Key() string {
	return "role:" + r.Name
}
