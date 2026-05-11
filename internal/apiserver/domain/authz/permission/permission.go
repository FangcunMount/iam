package permission

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
)

type Option func(*Permission)

func WithScope(s scope.Scope) Option {
	return func(p *Permission) {
		p.Scope = s.Normalized()
	}
}

// Permission states that a role can perform an action on a resource pattern.
type Permission struct {
	RoleName    role.Name
	TenantID    tenant.ID
	ResourceKey resource.Pattern
	Action      resource.ActionPattern
	Scope       scope.Scope
}

func New(roleName, tenantID, resourceKey, action string, opts ...Option) (Permission, error) {
	roleNameValue, err := role.NewName(roleName)
	if err != nil {
		return Permission{}, err
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Permission{}, err
	}
	resourcePattern, err := resource.NewPattern(resourceKey)
	if err != nil {
		return Permission{}, err
	}
	actionPattern, err := resource.NewActionPattern(action)
	if err != nil {
		return Permission{}, err
	}
	permission := Permission{
		RoleName:    roleNameValue,
		TenantID:    tenantIDValue,
		ResourceKey: resourcePattern,
		Action:      actionPattern,
		Scope:       scope.Default(),
	}
	for _, opt := range opts {
		opt(&permission)
	}
	if _, err := scope.New(permission.Scope.Kind, permission.Scope.Value); err != nil {
		return Permission{}, err
	}
	permission.Scope = permission.Scope.Normalized()
	return permission, nil
}

func (p Permission) RoleNameString() string {
	return p.RoleName.String()
}

func (p Permission) TenantIDString() string {
	return p.TenantID.String()
}

func (p Permission) ResourceKeyString() string {
	return p.ResourceKey.String()
}

func (p Permission) ActionString() string {
	return p.Action.String()
}
