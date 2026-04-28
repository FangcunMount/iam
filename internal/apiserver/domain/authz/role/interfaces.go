// Package role 角色领域包
package role

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Commander 角色命令服务接口（Driving Port - 写操作）
//
// 职责：
// - 处理角色的创建、更新、删除操作
// - 管理事务边界
// - 协调领域服务和仓储
type Commander interface {
	// CreateRole 创建角色
	CreateRole(ctx context.Context, cmd CreateRoleCommand) (*Role, error)

	// UpdateRole 更新角色
	UpdateRole(ctx context.Context, cmd UpdateRoleCommand) (*Role, error)

	// DeleteRole 删除角色
	DeleteRole(ctx context.Context, roleID meta.ID) error
}

// CreateRoleCommand 创建角色命令
type CreateRoleCommand struct {
	// Name 角色名称，在租户内唯一
	Name string

	// DisplayName 显示名称
	DisplayName string

	// TenantID 租户ID
	TenantID string

	// Description 角色描述
	Description string
}

// UpdateRoleCommand 更新角色命令
type UpdateRoleCommand struct {
	// ID 角色ID
	ID meta.ID

	// DisplayName 更新的显示名称（可选）
	DisplayName *string

	// Description 更新的描述（可选）
	Description *string
}

// Queryer 角色查询服务接口（Driving Port - 读操作）
//
// 职责：
// - 处理角色的查询操作
// - 提供不同维度的角色检索
// - 支持租户隔离的查询
type Queryer interface {
	// GetRoleByID 根据ID获取角色
	GetRoleByID(ctx context.Context, roleID meta.ID) (*Role, error)

	// GetRoleByName 根据名称获取角色（租户内）
	GetRoleByName(ctx context.Context, tenantID, name string) (*Role, error)

	// ListRoles 列出角色（支持分页和租户过滤）
	ListRoles(ctx context.Context, query ListRolesQuery) (*ListRolesResult, error)

	// ListRolesByTenant 列出指定租户的所有角色
	ListRolesByTenant(ctx context.Context, tenantID string) ([]*Role, error)
}

// ListRolesQuery 列出角色查询参数
type ListRolesQuery struct {
	// TenantID 租户ID过滤（可选）
	TenantID string

	// Offset 分页偏移量
	Offset int

	// Limit 分页限制（每页数量）
	Limit int
}

// ListRolesResult 列出角色结果
type ListRolesResult struct {
	// Roles 角色列表
	Roles []*Role

	// Total 总数量
	Total int64
}

// Validator 角色验证器接口（Driving Port - 领域服务）
// 封装角色相关的验证规则
type Validator interface {
	// ValidateCreateCommand 验证创建命令
	ValidateCreateCommand(cmd CreateRoleCommand) error

	// ValidateUpdateCommand 验证更新命令
	ValidateUpdateCommand(cmd UpdateRoleCommand) error

	// CheckNameUnique 检查名称唯一性
	CheckNameUnique(ctx context.Context, tenantID, name string) error
}
