package permission

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type Option func(*Permission)

func WithScope(s scope.Scope) Option {
	return func(p *Permission) {
		p.Scope = s.Normalized()
	}
}

// Permission states that a role can perform an action on a resource pattern.
type Permission struct {
	RoleName    string
	TenantID    string
	ResourceKey string
	Action      string
	Scope       scope.Scope
}

func New(roleName, tenantID, resourceKey, action string, opts ...Option) (Permission, error) {
	roleName = strings.TrimSpace(roleName)
	tenantID = strings.TrimSpace(tenantID)
	resourceKey = strings.TrimSpace(resourceKey)
	action = strings.TrimSpace(action)
	if _, err := role.NewName(roleName); err != nil {
		return Permission{}, err
	}
	if _, err := tenant.NewID(tenantID); err != nil {
		return Permission{}, err
	}
	if _, err := resource.NewPattern(resourceKey); err != nil {
		return Permission{}, err
	}
	if action == "" {
		return Permission{}, perrors.WithCode(code.ErrInvalidArgument, "action is required")
	}
	permission := Permission{
		RoleName:    roleName,
		TenantID:    tenantID,
		ResourceKey: resourceKey,
		Action:      action,
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
