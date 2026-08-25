// Package role 角色领域包
package role

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Repository 角色仓储接口（Driven Port）
type Repository interface {
	// Create 创建角色
	Create(ctx context.Context, role *Role) error
	// Update 更新角色
	Update(ctx context.Context, role *Role) error
	// Delete 删除角色
	Delete(ctx context.Context, id meta.ID) error
	// FindByID 根据ID获取角色
	FindByID(ctx context.Context, id meta.ID) (*Role, error)
	// FindByIDForUpdate locks the role aggregate until the surrounding transaction completes.
	FindByIDForUpdate(ctx context.Context, id meta.ID) (*Role, error)
	// FindByName 根据名称和租户获取角色
	FindByName(ctx context.Context, tenantID, name string) (*Role, error)
	// List 列出角色
	List(ctx context.Context, tenantID string, offset, limit int) ([]*Role, int64, error)
}
