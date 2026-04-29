package profile

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Repository Interface (Driven Port) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// Repository 档案存储接口
type Repository interface {
	Create(ctx context.Context, profile *Profile) error
	FindByID(ctx context.Context, id meta.ID) (*Profile, error)
	FindByName(ctx context.Context, name string) (*Profile, error)
	FindByIDCard(ctx context.Context, idCard meta.IDCard) (*Profile, error)
	FindListByName(ctx context.Context, name string) (profiles []*Profile, err error)
	FindListByNameAndBirthday(ctx context.Context, name string, birthday meta.Birthday) (profiles []*Profile, err error)
	FindSimilar(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) (profiles []*Profile, err error)
	Update(ctx context.Context, profile *Profile) error
}
