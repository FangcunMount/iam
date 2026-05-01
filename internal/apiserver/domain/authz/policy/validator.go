// Package policy 策略领域包
package policy

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// manager 策略管理器（领域服务实现）
// 封装策略相关的业务规则，包括：
// 1. 策略参数验证
// 2. 角色和资源的存在性检查
// 3. 租户隔离检查
// 4. Action 合法性验证
type validator struct {
	roleRepo     role.Repository
	resourceRepo resource.Repository
}

var _ Validator = (*validator)(nil)

// NewManager 创建策略管理器
func NewValidator(
	roleRepo role.Repository,
	resourceRepo resource.Repository,
) Validator {
	return &validator{
		roleRepo:     roleRepo,
		resourceRepo: resourceRepo,
	}
}

// ValidateAddPolicyParameters 验证添加策略参数
func (v *validator) ValidateAddPolicyParameters(
	roleID uint64,
	resourceID resource.ResourceID,
	action string,
	tenantID string,
	changedBy string,
) error {
	if roleID == 0 {
		return errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	if resourceID.Uint64() == 0 {
		return errors.WithCode(code.ErrInvalidArgument, "资源ID不能为空")
	}
	if action == "" {
		return errors.WithCode(code.ErrInvalidArgument, "Action 不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	if changedBy == "" {
		return errors.WithCode(code.ErrInvalidArgument, "变更人不能为空")
	}
	return nil
}

// ValidateRemovePolicyParameters 验证移除策略参数
func (v *validator) ValidateRemovePolicyParameters(
	roleID uint64,
	resourceID resource.ResourceID,
	action string,
	tenantID string,
	changedBy string,
) error {
	// 移除策略的参数验证与添加策略相同
	return v.ValidateAddPolicyParameters(roleID, resourceID, action, tenantID, changedBy)
}

// CheckRoleExistsAndTenant 检查角色是否存在并验证租户隔离。
// 返回业务角色名用于构造授权权限模型。
func (v *validator) CheckRoleExistsAndTenant(
	ctx context.Context,
	roleID uint64,
	tenantID string,
) (string, error) {
	id := meta.FromUint64(roleID) // roleID 来自请求，必定有效
	roleExists, err := v.roleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.IsCode(err, code.ErrRoleNotFound) {
			return "", errors.WithCode(code.ErrRoleNotFound, "角色 %d 不存在", roleID)
		}
		return "", errors.Wrap(err, "获取角色失败")
	}

	// 检查租户隔离
	if roleExists.TenantID != tenantID {
		return "", errors.WithCode(code.ErrPermissionDenied, "无权操作其他租户的角色")
	}

	return roleExists.Name, nil
}

// CheckResourceExistsAndValidateAction 检查资源是否存在并验证 Action 合法性
// 返回资源Key用于后续操作
func (v *validator) CheckResourceExistsAndValidateAction(
	ctx context.Context,
	resourceID resource.ResourceID,
	action string,
) (string, error) {
	return v.CheckResourceExistsActionAndScope(ctx, resourceID, action, authzDomain.DefaultScope())
}

func (v *validator) CheckResourceExistsActionAndScope(
	ctx context.Context,
	resourceID resource.ResourceID,
	action string,
	scope authzDomain.Scope,
) (string, error) {
	resourceExists, err := v.resourceRepo.FindByID(ctx, resourceID)
	if err != nil {
		if errors.IsCode(err, code.ErrResourceNotFound) {
			return "", errors.WithCode(code.ErrResourceNotFound, "资源 %d 不存在", resourceID.Uint64())
		}
		return "", errors.Wrap(err, "获取资源失败")
	}

	// 验证 Action 是否合法
	valid, err := v.resourceRepo.ValidateAction(ctx, resourceExists.Key, action)
	if err != nil {
		return "", errors.Wrap(err, "验证 Action 失败")
	}
	if !valid {
		return "", errors.WithCode(code.ErrInvalidAction, "Action %s 不被资源 %s 支持", action, resourceExists.Key)
	}
	if !resourceExists.AllowsScopeKind(scope.Normalized().Kind) {
		return "", errors.WithCode(code.ErrInvalidArgument, "资源 %s 不支持 scope %s", resourceExists.Key, scope.Normalized().Kind)
	}

	return resourceExists.Key, nil
}

// ValidateGetPoliciesQuery 验证获取策略查询参数
func (v *validator) ValidateGetPoliciesQuery(roleID uint64, tenantID string) error {
	if roleID == 0 {
		return errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

// ValidateGetVersionQuery 验证获取版本查询参数
func (v *validator) ValidateGetVersionQuery(tenantID string) error {
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}
