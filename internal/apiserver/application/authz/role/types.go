package role

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
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
	Name        roleDomain.Name
	DisplayName string
	TenantID    tenant.ID
	Description string
}

func NewCreateRoleCommand(name, displayName, tenantID, description string) (CreateRoleCommand, error) {
	roleName, err := roleDomain.NewName(name)
	if err != nil {
		return CreateRoleCommand{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return CreateRoleCommand{}, perrors.WithCode(code.ErrInvalidArgument, "显示名称不能为空")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return CreateRoleCommand{}, err
	}
	return CreateRoleCommand{
		Name:        roleName,
		DisplayName: displayName,
		TenantID:    tenantIDValue,
		Description: description,
	}, nil
}

func (cmd CreateRoleCommand) NameString() string {
	return cmd.Name.String()
}

func (cmd CreateRoleCommand) TenantIDString() string {
	return cmd.TenantID.String()
}

type UpdateRoleCommand struct {
	ID          meta.ID
	DisplayName *string
	Description *string
}

func NewUpdateRoleCommand(id meta.ID, displayName, description *string) (UpdateRoleCommand, error) {
	if id.IsZero() {
		return UpdateRoleCommand{}, perrors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	var displayNameValue *string
	if displayName != nil {
		trimmed := strings.TrimSpace(*displayName)
		if trimmed == "" {
			return UpdateRoleCommand{}, perrors.WithCode(code.ErrInvalidArgument, "显示名称不能为空")
		}
		displayNameValue = &trimmed
	}
	var descriptionValue *string
	if description != nil {
		value := *description
		descriptionValue = &value
	}
	return UpdateRoleCommand{
		ID:          id,
		DisplayName: displayNameValue,
		Description: descriptionValue,
	}, nil
}

type ListRolesQuery struct {
	TenantID tenant.ID
	Offset   int
	Limit    int
}

func NewListRolesQuery(tenantID string, offset, limit int) (ListRolesQuery, error) {
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return ListRolesQuery{}, err
	}
	return ListRolesQuery{
		TenantID: tenantIDValue,
		Offset:   offset,
		Limit:    limit,
	}, nil
}

func (query ListRolesQuery) TenantIDString() string {
	return query.TenantID.String()
}

type ListRolesResult struct {
	Roles []*roleDomain.Role
	Total int64
}
