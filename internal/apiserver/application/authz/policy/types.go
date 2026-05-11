package policy

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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
	Action     resourceDomain.Action
	Scope      scope.Scope
	TenantID   tenant.ID
	ChangedBy  policyDomain.Actor
	Reason     string
}

func NewAddPermissionCommand(roleID meta.ID, resourceID resourceDomain.ResourceID, action string, objectScope scope.Scope, tenantID, changedBy, reason string) (AddPermissionCommand, error) {
	cmd, err := newPermissionCommand(roleID, resourceID, action, objectScope, tenantID, changedBy, reason)
	if err != nil {
		return AddPermissionCommand{}, err
	}
	return AddPermissionCommand(cmd), nil
}

type RemovePermissionCommand struct {
	RoleID     meta.ID
	ResourceID resourceDomain.ResourceID
	Action     resourceDomain.Action
	Scope      scope.Scope
	TenantID   tenant.ID
	ChangedBy  policyDomain.Actor
	Reason     string
}

func NewRemovePermissionCommand(roleID meta.ID, resourceID resourceDomain.ResourceID, action string, objectScope scope.Scope, tenantID, changedBy, reason string) (RemovePermissionCommand, error) {
	cmd, err := newPermissionCommand(roleID, resourceID, action, objectScope, tenantID, changedBy, reason)
	if err != nil {
		return RemovePermissionCommand{}, err
	}
	return RemovePermissionCommand(cmd), nil
}

type permissionCommandShape struct {
	RoleID     meta.ID
	ResourceID resourceDomain.ResourceID
	Action     resourceDomain.Action
	Scope      scope.Scope
	TenantID   tenant.ID
	ChangedBy  policyDomain.Actor
	Reason     string
}

func newPermissionCommand(roleID meta.ID, resourceID resourceDomain.ResourceID, action string, objectScope scope.Scope, tenantID, changedBy, reason string) (permissionCommandShape, error) {
	if roleID.IsZero() {
		return permissionCommandShape{}, errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	if resourceID.Uint64() == 0 {
		return permissionCommandShape{}, errors.WithCode(code.ErrInvalidArgument, "资源ID不能为空")
	}
	actionValue, err := resourceDomain.NewAction(action)
	if err != nil {
		return permissionCommandShape{}, err
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return permissionCommandShape{}, err
	}
	normalizedScope := objectScope.Normalized()
	if _, err := scope.New(normalizedScope.Kind, normalizedScope.Value); err != nil {
		return permissionCommandShape{}, err
	}
	actor, err := policyDomain.NewActor(changedBy)
	if err != nil {
		return permissionCommandShape{}, err
	}
	return permissionCommandShape{
		RoleID:     roleID,
		ResourceID: resourceID,
		Action:     actionValue,
		Scope:      normalizedScope,
		TenantID:   tenantIDValue,
		ChangedBy:  actor,
		Reason:     reason,
	}, nil
}

func (cmd AddPermissionCommand) ActionString() string {
	return cmd.Action.String()
}

func (cmd AddPermissionCommand) TenantIDString() string {
	return cmd.TenantID.String()
}

func (cmd AddPermissionCommand) ChangedByString() string {
	return cmd.ChangedBy.String()
}

func (cmd RemovePermissionCommand) ActionString() string {
	return cmd.Action.String()
}

func (cmd RemovePermissionCommand) TenantIDString() string {
	return cmd.TenantID.String()
}

func (cmd RemovePermissionCommand) ChangedByString() string {
	return cmd.ChangedBy.String()
}

type RolePermissionsQuery struct {
	RoleID   meta.ID
	TenantID string
}

type CurrentVersionQuery struct {
	TenantID string
}
