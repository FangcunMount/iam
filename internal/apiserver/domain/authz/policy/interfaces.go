// Package policy 策略领域包
package policy

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Validator 策略验证器接口。
// 封装策略相关的验证规则
type Validator interface {
	// ValidateAddPolicyParameters 验证添加策略参数
	ValidateAddPolicyParameters(
		roleID meta.ID,
		resourceID resource.ResourceID,
		action string,
		tenantID string,
		changedBy string,
	) error

	// ValidateRemovePolicyParameters 验证移除策略参数
	ValidateRemovePolicyParameters(
		roleID meta.ID,
		resourceID resource.ResourceID,
		action string,
		tenantID string,
		changedBy string,
	) error

	// CheckRoleExistsAndTenant 检查角色是否存在并验证租户隔离。
	// 返回业务角色名，用于构造授权权限模型。
	CheckRoleExistsAndTenant(
		ctx context.Context,
		roleID meta.ID,
		tenantID string,
	) (string, error)

	// CheckResourceExistsAndValidateAction 检查资源是否存在并验证 Action 合法性
	// 返回资源 Key 用于后续操作
	CheckResourceExistsAndValidateAction(
		ctx context.Context,
		resourceID resource.ResourceID,
		action string,
	) (string, error) // 返回 resource key

	// CheckResourceExistsActionAndScope 检查资源存在、Action 合法且资源支持目标 scope。
	CheckResourceExistsActionAndScope(
		ctx context.Context,
		resourceID resource.ResourceID,
		action string,
		scope scope.Scope,
	) (string, error)
}
