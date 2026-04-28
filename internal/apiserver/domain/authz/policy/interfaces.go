// Package policy 策略领域包
package policy

import (
	"context"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
)

// Commander 策略命令接口（Driving Port - 领域服务）
// 定义策略管理的用例接口，遵循 CQRS 原则
type Commander interface {
	// AddPolicyRule 添加策略规则
	AddPolicyRule(ctx context.Context, cmd AddPolicyRuleCommand) error

	// RemovePolicyRule 移除策略规则
	RemovePolicyRule(ctx context.Context, cmd RemovePolicyRuleCommand) error
}

// AddPolicyRuleCommand 添加策略规则命令
type AddPolicyRuleCommand struct {
	RoleID     uint64              // 角色ID
	ResourceID resource.ResourceID // 资源ID
	Action     string              // 操作
	TenantID   string              // 租户ID
	ChangedBy  string              // 变更人
	Reason     string              // 变更原因
}

// RemovePolicyRuleCommand 移除策略规则命令
type RemovePolicyRuleCommand struct {
	RoleID     uint64              // 角色ID
	ResourceID resource.ResourceID // 资源ID
	Action     string              // 操作
	TenantID   string              // 租户ID
	ChangedBy  string              // 变更人
	Reason     string              // 变更原因
}

// Queryer 策略查询接口（Driving Port - 领域服务）
// 定义策略查询的用例接口，遵循 CQRS 原则
type Queryer interface {
	// GetPoliciesByRole 获取角色的所有策略规则
	GetPoliciesByRole(ctx context.Context, query GetPoliciesByRoleQuery) ([]PolicyRule, error)

	// GetCurrentVersion 获取当前策略版本
	GetCurrentVersion(ctx context.Context, query GetCurrentVersionQuery) (*PolicyVersion, error)
}

// GetPoliciesByRoleQuery 获取角色策略查询
type GetPoliciesByRoleQuery struct {
	RoleID   uint64 // 角色ID
	TenantID string // 租户ID
}

// GetCurrentVersionQuery 获取当前版本查询
type GetCurrentVersionQuery struct {
	TenantID string // 租户ID
}

// BuildPolicyRule 构建策略规则（辅助方法）
func BuildPolicyRule(roleKey, tenantID, resourceKey, action string) PolicyRule {
	return PolicyRule{
		Sub: roleKey,
		Dom: tenantID,
		Obj: resourceKey,
		Act: action,
	}
}

// Validator 策略验证器接口（Driving Port - 领域服务）
// 封装策略相关的验证规则
type Validator interface {
	// ValidateAddPolicyParameters 验证添加策略参数
	ValidateAddPolicyParameters(
		roleID uint64,
		resourceID resource.ResourceID,
		action string,
		tenantID string,
		changedBy string,
	) error

	// ValidateRemovePolicyParameters 验证移除策略参数
	ValidateRemovePolicyParameters(
		roleID uint64,
		resourceID resource.ResourceID,
		action string,
		tenantID string,
		changedBy string,
	) error

	// CheckRoleExistsAndTenant 检查角色是否存在并验证租户隔离
	// 返回角色 Key 用于后续操作
	CheckRoleExistsAndTenant(
		ctx context.Context,
		roleID uint64,
		tenantID string,
	) (string, error) // 返回 role key

	// CheckResourceExistsAndValidateAction 检查资源是否存在并验证 Action 合法性
	// 返回资源 Key 用于后续操作
	CheckResourceExistsAndValidateAction(
		ctx context.Context,
		resourceID resource.ResourceID,
		action string,
	) (string, error) // 返回 resource key
}
