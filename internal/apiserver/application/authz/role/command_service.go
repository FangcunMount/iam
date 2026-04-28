// Package role 角色应用服务
package role

import (
	"context"

	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// RoleCommandService 角色命令服务（写操作）
type RoleCommandService struct {
	roleValidator roleDomain.Validator
	roleRepo      roleDomain.Repository
}

// NewRoleCommandService 创建角色命令服务
func NewRoleCommandService(
	roleValidator roleDomain.Validator,
	roleRepo roleDomain.Repository,
) *RoleCommandService {
	return &RoleCommandService{
		roleValidator: roleValidator,
		roleRepo:      roleRepo,
	}
}

// CreateRole 创建角色
func (s *RoleCommandService) CreateRole(
	ctx context.Context,
	cmd roleDomain.CreateRoleCommand,
) (*roleDomain.Role, error) {
	// 1. 验证创建命令
	if err := s.roleValidator.ValidateCreateCommand(cmd); err != nil {
		return nil, err
	}

	// 2. 创建角色领域对象
	newRole := roleDomain.NewRole(
		cmd.Name,
		cmd.DisplayName,
		cmd.TenantID,
		roleDomain.WithDescription(cmd.Description),
	)

	// 3. 持久化到仓储
	if err := s.roleRepo.Create(ctx, &newRole); err != nil {
		return nil, err
	}

	return &newRole, nil
}

// UpdateRole 更新角色
func (s *RoleCommandService) UpdateRole(
	ctx context.Context,
	cmd roleDomain.UpdateRoleCommand,
) (*roleDomain.Role, error) {
	// 1. 验证更新命令
	if err := s.roleValidator.ValidateUpdateCommand(cmd); err != nil {
		return nil, err
	}

	// 2. 获取角色
	existingRole, err := s.roleRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	// 3. 更新领域对象属性
	if cmd.DisplayName != nil {
		existingRole.DisplayName = *cmd.DisplayName
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
func (s *RoleCommandService) DeleteRole(
	ctx context.Context,
	roleID meta.ID,
) error {
	// 直接删除角色（Repository 会处理不存在的情况）
	return s.roleRepo.Delete(ctx, roleID)
}
