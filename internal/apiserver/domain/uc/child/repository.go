package child

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Repository Interface (Driven Port) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// Repository 儿童档案存储接口
type Repository interface {
	Create(ctx context.Context, child *Child) error
	FindByID(ctx context.Context, id meta.ID) (*Child, error)
	FindByName(ctx context.Context, name string) (*Child, error)
	FindByIDCard(ctx context.Context, idCard meta.IDCard) (*Child, error)
	FindListByName(ctx context.Context, name string) (children []*Child, err error)
	FindListByNameAndBirthday(ctx context.Context, name string, birthday meta.Birthday) (children []*Child, err error)
	FindSimilar(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) (children []*Child, err error)
	Update(ctx context.Context, child *Child) error
}
