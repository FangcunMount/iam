package guardianship

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Repository Interface (Driven Port) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// Repository 监护关系存储接口
type Repository interface {
	Create(ctx context.Context, guardianship *Guardianship) error
	FindByID(ctx context.Context, id meta.ID) (*Guardianship, error)
	FindByChildID(ctx context.Context, id meta.ID) (guardianships []*Guardianship, err error)
	FindByChildIDIncludingRevoked(ctx context.Context, id meta.ID) (guardianships []*Guardianship, err error)
	FindByUserID(ctx context.Context, id meta.ID) (guardianships []*Guardianship, err error)
	FindByUserIDIncludingRevoked(ctx context.Context, id meta.ID) (guardianships []*Guardianship, err error)
	FindByUserIDAndChildID(ctx context.Context, userID meta.ID, childID meta.ID) (*Guardianship, error)
	FindByUserIDAndChildIDIncludingRevoked(ctx context.Context, userID meta.ID, childID meta.ID) (*Guardianship, error)
	IsGuardian(ctx context.Context, userID meta.ID, childID meta.ID) (bool, error)
	Update(ctx context.Context, guardianship *Guardianship) error
}
