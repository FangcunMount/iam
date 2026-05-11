// Package service 角色领域服务
//
// 本包提供角色管理的领域服务，封装业务规则。
// 领域服务是内部实现细节，不对外暴露，仅被应用服务编排使用。
package role

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// RoleManager 角色管理领域服务
//
// 职责：
// - 封装角色相关的业务规则
// - 提供角色名称唯一性检查、租户隔离等业务逻辑
// - 被应用服务编排使用，不对接口层直接暴露
//
// 设计原则：
// - 不实现 driving 接口（那是应用服务的职责）
// - 提供细粒度的业务规则方法
// - 无状态，所有依赖通过构造函数注入
type validator struct {
	roleRepo Repository
}

// NewValidator 创建角色验证器
func NewValidator(roleRepo Repository) *validator {
	return &validator{
		roleRepo: roleRepo,
	}
}

// CheckNameUnique 检查角色名称在租户内的唯一性
//
// 业务规则：角色名称在同一租户内必须唯一
func (v *validator) CheckNameUnique(ctx context.Context, tenantID, name string) error {
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	if name == "" {
		return errors.WithCode(code.ErrInvalidArgument, "角色名称不能为空")
	}

	// 查询是否已存在
	existingRole, err := v.roleRepo.FindByName(ctx, tenantID, name)
	if err != nil && !errors.IsCode(err, code.ErrRoleNotFound) {
		return errors.Wrap(err, "检查角色名称唯一性失败")
	}

	if existingRole != nil {
		return errors.WithCode(code.ErrRoleAlreadyExists, "角色名称 %s 在租户 %s 中已存在", name, tenantID)
	}

	return nil
}

// ValidateCreateParameters 验证创建角色的参数
//
// 业务规则：
// - Name 不能为空
// - DisplayName 不能为空
// - TenantID 不能为空
func (v *validator) ValidateCreateParameters(name, displayName, tenantID string) error {
	if name == "" {
		return errors.WithCode(code.ErrInvalidArgument, "角色名称不能为空")
	}
	if displayName == "" {
		return errors.WithCode(code.ErrInvalidArgument, "显示名称不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

// CheckRoleExists 检查角色是否存在
func (v *validator) CheckRoleExists(ctx context.Context, roleID meta.ID) (*Role, error) {
	if roleID.Uint64() == 0 {
		return nil, errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}

	foundRole, err := v.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		if errors.IsCode(err, code.ErrRoleNotFound) {
			return nil, errors.WithCode(code.ErrRoleNotFound, "角色 %d 不存在", roleID.Uint64())
		}
		return nil, errors.Wrap(err, "获取角色失败")
	}

	return foundRole, nil
}

// CheckTenantOwnership 检查角色是否属于指定租户
//
// 业务规则：租户隔离，只能操作自己租户的角色
func (v *validator) CheckTenantOwnership(roleEntity *Role, tenantID string) error {
	if roleEntity == nil {
		return errors.WithCode(code.ErrInvalidArgument, "角色对象不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}

	if !roleEntity.BelongsToTenant(tenantID) {
		return errors.WithCode(code.ErrPermissionDenied, "无权访问其他租户的角色")
	}

	return nil
}
