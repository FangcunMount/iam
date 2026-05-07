package policy

import (
	"context"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type PermissionCommands interface {
	AddPermission(ctx context.Context, cmd AddPermissionCommand) error
	RemovePermission(ctx context.Context, cmd RemovePermissionCommand) error
}

type PermissionReader interface {
	GetPermissionsForRole(ctx context.Context, query RolePermissionsQuery) ([]authzDomain.Permission, error)
	GetCurrentVersion(ctx context.Context, query CurrentVersionQuery) (*policyDomain.PolicyVersion, error)
}

type RolePermissionStore interface {
	PermissionsForRole(ctx context.Context, roleName, tenantID string) ([]authzDomain.Permission, error)
}

type AddPermissionCommand struct {
	RoleID     meta.ID
	ResourceID resourceDomain.ResourceID
	Action     string
	Scope      authzDomain.Scope
	TenantID   string
	ChangedBy  string
	Reason     string
}

type RemovePermissionCommand struct {
	RoleID     meta.ID
	ResourceID resourceDomain.ResourceID
	Action     string
	Scope      authzDomain.Scope
	TenantID   string
	ChangedBy  string
	Reason     string
}

type RolePermissionsQuery struct {
	RoleID   meta.ID
	TenantID string
}

type CurrentVersionQuery struct {
	TenantID string
}
