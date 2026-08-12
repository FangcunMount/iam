// Package role 角色应用服务
package role

import (
	"context"

	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// RoleCatalog manages the role catalog inside a tenant.
type RoleCatalog struct {
	roleValidator roleDomain.Validator
	roleRepo      roleDomain.Repository
}

func NewRoleCatalog(
	roleValidator roleDomain.Validator,
	roleRepo roleDomain.Repository,
) *RoleCatalog {
	return &RoleCatalog{
		roleValidator: roleValidator,
		roleRepo:      roleRepo,
	}
}

// CreateRole 创建角色
func (s *RoleCatalog) CreateRole(
	ctx context.Context,
	cmd CreateRoleCommand,
) (*roleDomain.Role, error) {
	// 1. 验证创建命令
	if err := s.roleValidator.ValidateCreateParameters(cmd.NameString(), cmd.DisplayName, cmd.TenantIDString()); err != nil {
		return nil, err
	}

	// 2. 创建角色领域对象
	newRole, err := roleDomain.NewRole(
		cmd.NameString(),
		cmd.DisplayName,
		cmd.TenantIDString(),
		roleDomain.WithDescription(cmd.Description),
	)
	if err != nil {
		return nil, err
	}

	// 3. 持久化到仓储
	if err := s.roleRepo.Create(ctx, &newRole); err != nil {
		return nil, err
	}

	return &newRole, nil
}

// UpdateRole 更新角色
func (s *RoleCatalog) UpdateRole(
	ctx context.Context,
	cmd UpdateRoleCommand,
) (*roleDomain.Role, error) {
	// 1. 验证更新命令
	// 2. 获取角色
	existingRole, err := s.roleRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	// 3. 更新领域对象属性
	if cmd.DisplayName != nil {
		if err := existingRole.Rename(*cmd.DisplayName); err != nil {
			return nil, err
		}
	}
	if cmd.Description != nil {
		existingRole.Description = *cmd.Description
	}

	// 4. 持久化更新
	if err := s.roleRepo.Update(ctx, existingRole); err != nil {
		return nil, err
	}

	return existingRole, nil
}

// DeleteRole 删除角色
func (s *RoleCatalog) DeleteRole(
	ctx context.Context,
	roleID meta.ID,
) error {
	// 直接删除角色（Repository 会处理不存在的情况）
	return s.roleRepo.Delete(ctx, roleID)
}
