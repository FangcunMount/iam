package user

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Repository Interface (Driven Port) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// Repository 用户存储接口
// 由 infrastructure 层实现，领域服务通过此接口访问数据
type Repository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id meta.ID) (*User, error)
	FindByPhone(ctx context.Context, phone meta.Phone) (*User, error)
	Update(ctx context.Context, user *User) error
}
