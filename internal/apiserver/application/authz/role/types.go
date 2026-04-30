package role

import (
	"context"

	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type Catalog interface {
	CreateRole(ctx context.Context, cmd CreateRoleCommand) (*roleDomain.Role, error)
	UpdateRole(ctx context.Context, cmd UpdateRoleCommand) (*roleDomain.Role, error)
	DeleteRole(ctx context.Context, roleID meta.ID) error
}

type Directory interface {
	GetRoleByID(ctx context.Context, roleID meta.ID) (*roleDomain.Role, error)
	GetRoleByName(ctx context.Context, tenantID, name string) (*roleDomain.Role, error)
	ListRoles(ctx context.Context, query ListRolesQuery) (*ListRolesResult, error)
	ListRolesByTenant(ctx context.Context, tenantID string) ([]*roleDomain.Role, error)
}

type CreateRoleCommand struct {
	Name        string
	DisplayName string
	TenantID    string
	Description string
}

type UpdateRoleCommand struct {
	ID          meta.ID
	DisplayName *string
	Description *string
}

type ListRolesQuery struct {
	TenantID string
	Offset   int
	Limit    int
}

type ListRolesResult struct {
	Roles []*roleDomain.Role
	Total int64
}
