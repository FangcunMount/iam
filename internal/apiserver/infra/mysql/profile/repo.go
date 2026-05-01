package profile

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profile"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profile"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"gorm.io/gorm"
)

// Repository 档案存储库实现
type Repository struct {
	mysql.BaseRepository[*ProfilePO]
	mapper *ProfileMapper
}

// NewRepository 创建档案存储库
func NewRepository(db *gorm.DB) profile.Repository {
	base := mysql.NewBaseRepository[*ProfilePO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrIdentityProfileExists, "profile already exists")
	}))

	return &Repository{
		BaseRepository: base,
		mapper:         NewProfileMapper(),
	}
}

// Create 创建新的档案
func (r *Repository) Create(ctx context.Context, profile *domain.Profile) error {
	po := r.mapper.ToPO(profile)
	return r.CreateAndSync(ctx, po, func(updated *ProfilePO) {
		profile.ID = updated.ID
	})
}

// FindByID 根据 ID 查找档案
func (r *Repository) FindByID(ctx context.Context, id meta.ID) (*domain.Profile, error) {
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

// FindByName 根据姓名查找档案
func (r *Repository) FindByName(ctx context.Context, name string) (*domain.Profile, error) {
	var po ProfilePO
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

// FindByIDCard 根据身份证号查找档案
func (r *Repository) FindByIDCard(ctx context.Context, idCard meta.IDCard) (*domain.Profile, error) {
	var po ProfilePO
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

// FindListByName 根据姓名查找档案列表
func (r *Repository) FindListByName(ctx context.Context, name string) ([]*domain.Profile, error) {
	var pos []*ProfilePO
	if err := r.WithContext(ctx).Where("name = ?", name).Find(&pos).Error; err != nil {
		return nil, err
	}
	return r.toProfiles(pos), nil
}

// FindListByNameAndBirthday 根据姓名和生日查找档案列表
func (r *Repository) FindListByNameAndBirthday(ctx context.Context, name string, birthday meta.Birthday) ([]*domain.Profile, error) {
	var pos []*ProfilePO
	db := r.WithContext(ctx).Where("name = ?", name)
	if !birthday.IsEmpty() {
		db = db.Where("birthday = ?", birthday.String())
	}
	if err := db.Find(&pos).Error; err != nil {
		return nil, err
	}
	return r.toProfiles(pos), nil
}

// FindSimilar 根据姓名 + 性别 + 出生日期查找相似档案
func (r *Repository) FindSimilar(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) ([]*domain.Profile, error) {
	var pos []*ProfilePO

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

	return r.toProfiles(pos), nil
}

func (r *Repository) toProfiles(pos []*ProfilePO) []*domain.Profile {
	bos := r.mapper.ToBOs(pos)
	profiles := make([]*domain.Profile, 0, len(bos))
	for _, bo := range bos {
		if bo == nil {
			continue
		}
		profiles = append(profiles, bo)
	}

	return profiles
}

// Update 更新档案信息
func (r *Repository) Update(ctx context.Context, profile *domain.Profile) error {
	po := r.mapper.ToPO(profile)
	return r.UpdateAndSync(ctx, po, func(updated *ProfilePO) {
		profile.ID = updated.ID
	})
}
