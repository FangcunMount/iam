package role

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Role 角色领域对象（聚合根）
type Role struct {
	ID          meta.ID
	Name        Name   // 角色名称
	DisplayName string // 显示名称
	TenantID    tenant.ID
	Description string // 描述
}

// NewRole 创建新角色。
func NewRole(name, displayName, tenantID string, opts ...RoleOption) (Role, error) {
	roleName, err := NewName(name)
	if err != nil {
		return Role{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return Role{}, perrors.WithCode(code.ErrInvalidArgument, "显示名称不能为空")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Role{}, err
	}
	role := Role{
		Name:        roleName,
		DisplayName: displayName,
		TenantID:    tenantIDValue,
	}
	for _, opt := range opts {
		opt(&role)
	}
	return role, nil
}

// RoleOption 角色选项
type RoleOption func(*Role)

func WithID(id meta.ID) RoleOption           { return func(r *Role) { r.ID = id } }
func WithDescription(desc string) RoleOption { return func(r *Role) { r.Description = desc } }

func (r Role) BelongsToTenant(tenantID string) bool {
	return r.TenantIDString() == strings.TrimSpace(tenantID)
}

func (r Role) NameString() string {
	return r.Name.String()
}

func (r Role) TenantIDString() string {
	return r.TenantID.String()
}

func (r *Role) Rename(displayName string) error {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "显示名称不能为空")
	}
	r.DisplayName = displayName
	return nil
}

func (r *Role) ChangeDescription(description string) {
	r.Description = description
}
