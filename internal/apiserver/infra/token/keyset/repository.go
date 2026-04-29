package jwks

import (
	"context"
)

// ================== Repository Interface (Driven Port) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// Repository 密钥仓储接口
// 负责 Key 实体的持久化操作（MySQL/PostgreSQL 等）
type Repository interface {
	// Save 保存新密钥（Create）
	Save(ctx context.Context, key *Key) error

	// Update 更新已有密钥（Update status, NotAfter 等）
	Update(ctx context.Context, key *Key) error

	// Delete 删除密钥（物理删除）
	Delete(ctx context.Context, kid string) error

	// FindByKid 根据 kid 查询单个密钥
	FindByKid(ctx context.Context, kid string) (*Key, error)

	// FindByStatus 根据状态查询密钥列表
	// status: "active", "grace", "retired"
	FindByStatus(ctx context.Context, status KeyStatus) ([]*Key, error)

	// FindPublishable 查询可发布的密钥（Active + Grace 状态且未过期）
	// 用于生成 /.well-known/jwks.json
	FindPublishable(ctx context.Context) ([]*Key, error)

	// FindExpired 查询已过期的密钥（NotAfter < now）
	// 用于清理任务
	FindExpired(ctx context.Context) ([]*Key, error)

	// FindAll 查询所有密钥（分页）
	FindAll(ctx context.Context, limit, offset int) ([]*Key, int64, error)

	// CountByStatus 统计指定状态的密钥数量
	CountByStatus(ctx context.Context, status KeyStatus) (int64, error)
}
