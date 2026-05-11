package policy

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type PermissionCommands interface {
	AddPermission(ctx context.Context, cmd AddPermissionCommand) error
	RemovePermission(ctx context.Context, cmd RemovePermissionCommand) error
}

type PermissionReader interface {
	GetPermissionsForRole(ctx context.Context, query RolePermissionsQuery) ([]permission.Permission, error)
	GetCurrentVersion(ctx context.Context, query CurrentVersionQuery) (*policyDomain.PolicyVersion, error)
}

type RolePermissionStore interface {
	PermissionsForRole(ctx context.Context, roleName, tenantID string) ([]permission.Permission, error)
}

type AddPermissionCommand struct {
	RoleID     meta.ID
	ResourceID resourceDomain.ResourceID
	Action     string
	Scope      scope.Scope
	TenantID   string
	ChangedBy  string
	Reason     string
}

type RemovePermissionCommand struct {
	RoleID     meta.ID
	ResourceID resourceDomain.ResourceID
	Action     string
	Scope      scope.Scope
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
