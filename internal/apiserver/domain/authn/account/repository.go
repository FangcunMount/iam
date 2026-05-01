package account

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ==================== Driven Ports (被驱动端口) ====================
// 由基础设施层实现，领域层使用

// Repository 账号仓储接口（Driven Port）
// 职责：账号持久化操作
type Repository interface {
	// Create 创建账号
	Create(ctx context.Context, a *Account) error

	// Update*** 更新账号信息
	UpdateUniqueID(ctx context.Context, id meta.ID, uniqueID UnionID) error
	UpdateStatus(ctx context.Context, id meta.ID, status AccountStatus) error
	UpdateProfile(ctx context.Context, id meta.ID, profile map[string]string) error
	UpdateMeta(ctx context.Context, id meta.ID, meta map[string]string) error

	// GetBy*** 查询账号
	GetByID(ctx context.Context, id meta.ID) (*Account, error)
	GetByUniqueID(ctx context.Context, uniqueID UnionID) (*Account, error)
	GetByExternalIDAppId(ctx context.Context, externalID ExternalID, appID AppId) (*Account, error)
}
