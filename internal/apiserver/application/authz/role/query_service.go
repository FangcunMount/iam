// Package role 角色应用服务
package role

import (
	"context"

	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// RoleQueryService 角色查询服务（读操作）
type RoleQueryService struct {
	roleRepo roleDomain.Repository
}

// NewRoleQueryService 创建角色查询服务
func NewRoleQueryService(
	roleRepo roleDomain.Repository,
) *RoleQueryService {
	return &RoleQueryService{
		roleRepo: roleRepo,
	}
}

// GetRoleByID 根据ID获取角色
func (s *RoleQueryService) GetRoleByID(
	ctx context.Context,
	roleID meta.ID,
) (*roleDomain.Role, error) {
	return s.roleRepo.FindByID(ctx, roleID)
}

// GetRoleByName 根据名称获取角色（租户内）
func (s *RoleQueryService) GetRoleByName(
	ctx context.Context,
	tenantID, name string,
) (*roleDomain.Role, error) {
	return s.roleRepo.FindByName(ctx, tenantID, name)
}

// ListRoles 列出角色（支持分页和租户过滤）
func (s *RoleQueryService) ListRoles(
	ctx context.Context,
	query roleDomain.ListRolesQuery,
) (*roleDomain.ListRolesResult, error) {
	roles, total, err := s.roleRepo.List(ctx, query.TenantID, query.Offset, query.Limit)
	if err != nil {
		return nil, err
	}

	return &roleDomain.ListRolesResult{
		Roles: roles,
		Total: total,
	}, nil
}

// ListRolesByTenant 列出指定租户的所有角色
func (s *RoleQueryService) ListRolesByTenant(
	ctx context.Context,
	tenantID string,
) ([]*roleDomain.Role, error) {
	// 不限制分页，返回所有角色
	roles, _, err := s.roleRepo.List(ctx, tenantID, 0, -1)
	return roles, err
}
