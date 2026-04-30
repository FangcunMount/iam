// Package resource 资源领域包
package resource

import (
	"context"
)

// Repository 资源目录仓储接口（Driven Port）
type Repository interface {
	// Create 创建资源
	Create(ctx context.Context, resource *Resource) error
	// Update 更新资源
	Update(ctx context.Context, resource *Resource) error
	// Delete 删除资源
	Delete(ctx context.Context, id ResourceID) error
	// FindByID 根据ID获取资源
	FindByID(ctx context.Context, id ResourceID) (*Resource, error)
	// FindByKey 根据键获取资源
	FindByKey(ctx context.Context, key string) (*Resource, error)
	// List 列出资源（支持过滤和分页）
	List(ctx context.Context, query ResourceFilter) ([]*Resource, int64, error)
	// ValidateAction 校验动作是否在资源的允许列表中
	ValidateAction(ctx context.Context, resourceKey, action string) (bool, error)
}
