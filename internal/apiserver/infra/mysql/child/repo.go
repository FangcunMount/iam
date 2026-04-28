package child

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// Repository 儿童档案存储库实现
type Repository struct {
	mysql.BaseRepository[*ChildPO]
	mapper *ChildMapper
}

// NewRepository 创建儿童档案存储库
func NewRepository(db *gorm.DB) child.Repository {
	base := mysql.NewBaseRepository[*ChildPO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrIdentityChildExists, "child already exists")
	}))

	return &Repository{
		BaseRepository: base,
		mapper:         NewChildMapper(),
	}
}

// Create 创建新的儿童档案
func (r *Repository) Create(ctx context.Context, child *domain.Child) error {
	po := r.mapper.ToPO(child)
	return r.CreateAndSync(ctx, po, func(updated *ChildPO) {
		child.ID = updated.ID
	})
}

// FindByID 根据 ID 查找儿童档案
func (r *Repository) FindByID(ctx context.Context, id meta.ID) (*domain.Child, error) {
	po, err := r.BaseRepository.FindByID(ctx, id.Uint64())
	if err != nil {
		return nil, err
	}
	c := r.mapper.ToBO(po)
	if c == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}

// FindByName 根据姓名查找儿童档案
func (r *Repository) FindByName(ctx context.Context, name string) (*domain.Child, error) {
	var po ChildPO
	err := r.FindByField(ctx, &po, "name", name)
	if err != nil {
		return nil, err
	}
	c := r.mapper.ToBO(&po)
	if c == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}

// FindByIDCard 根据身份证号查找儿童档案
func (r *Repository) FindByIDCard(ctx context.Context, idCard meta.IDCard) (*domain.Child, error) {
	var po ChildPO
	err := r.FindByField(ctx, &po, "id_card", idCard.String())
	if err != nil {
		return nil, err
	}
	c := r.mapper.ToBO(&po)
	if c == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}

// FindListByName 根据姓名查找儿童档案列表
func (r *Repository) FindListByName(ctx context.Context, name string) ([]*domain.Child, error) {
	var pos []*ChildPO
	if err := r.WithContext(ctx).Where("name = ?", name).Find(&pos).Error; err != nil {
		return nil, err
	}
	return r.toChildren(pos), nil
}

// FindListByNameAndBirthday 根据姓名和生日查找儿童档案列表
func (r *Repository) FindListByNameAndBirthday(ctx context.Context, name string, birthday meta.Birthday) ([]*domain.Child, error) {
	var pos []*ChildPO
	db := r.WithContext(ctx).Where("name = ?", name)
	if !birthday.IsEmpty() {
		db = db.Where("birthday = ?", birthday.String())
	}
	if err := db.Find(&pos).Error; err != nil {
		return nil, err
	}
	return r.toChildren(pos), nil
}

// FindSimilar 根据姓名 + 性别 + 出生日期查找相似档案
func (r *Repository) FindSimilar(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) ([]*domain.Child, error) {
	var pos []*ChildPO

	db := r.WithContext(ctx)
	if name != "" {
		db = db.Where("name = ?", name)
	}
	if gender.Value() != 0 {
		db = db.Where("gender = ?", gender.Value())
	}
	if !birthday.IsEmpty() {
		db = db.Where("birthday = ?", birthday.String())
	}

	if err := db.Find(&pos).Error; err != nil {
		return nil, err
	}

	return r.toChildren(pos), nil
}

func (r *Repository) toChildren(pos []*ChildPO) []*domain.Child {
	bos := r.mapper.ToBOs(pos)
	children := make([]*domain.Child, 0, len(bos))
	for _, bo := range bos {
		if bo == nil {
			continue
		}
		children = append(children, bo)
	}

	return children
}

// Update 更新儿童档案信息
func (r *Repository) Update(ctx context.Context, child *domain.Child) error {
	po := r.mapper.ToPO(child)
	return r.UpdateAndSync(ctx, po, func(updated *ChildPO) {
		child.ID = updated.ID
	})
}
