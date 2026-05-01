package profilelink

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ================== Repository Interface (Driven Port) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// Repository 档案关系存储接口
type Repository interface {
	Create(ctx context.Context, profileLink *ProfileLink) error
	FindByID(ctx context.Context, id meta.ID) (*ProfileLink, error)
	FindByProfileID(ctx context.Context, id meta.ID) (profileLinks []*ProfileLink, err error)
	FindByProfileIDIncludingRevoked(ctx context.Context, id meta.ID) (profileLinks []*ProfileLink, err error)
	FindByUserID(ctx context.Context, id meta.ID) (profileLinks []*ProfileLink, err error)
	FindByUserIDIncludingRevoked(ctx context.Context, id meta.ID) (profileLinks []*ProfileLink, err error)
	FindByUserIDAndProfileID(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error)
	FindByUserIDAndProfileIDIncludingRevoked(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error)
	IsLinked(ctx context.Context, userID meta.ID, profileID meta.ID) (bool, error)
	Update(ctx context.Context, profileLink *ProfileLink) error
}
