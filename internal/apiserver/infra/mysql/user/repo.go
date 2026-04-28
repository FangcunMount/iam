package user

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// Repository 用户存储库实现
type Repository struct {
	mysql.BaseRepository[*UserPO]
	mapper *UserMapper
}

// NewRepository 创建用户存储库
func NewRepository(db *gorm.DB) user.Repository {
	base := mysql.NewBaseRepository[*UserPO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrUserAlreadyExists, "user already exists")
	}))

	return &Repository{
		BaseRepository: base,
		mapper:         NewUserMapper(),
	}
}

// Create 创建新用户
func (r *Repository) Create(ctx context.Context, u *domain.User) error {
	po := r.mapper.ToPO(u)
	return r.CreateAndSync(ctx, po, func(updated *UserPO) {
		id := meta.FromUint64(updated.ID.Uint64()) // ID 来自数据库，必定有效
		u.ID = id
	})
}

// FindByID 根据ID查找用户
func (r *Repository) FindByID(ctx context.Context, id meta.ID) (*domain.User, error) {
	po, err := r.BaseRepository.FindByID(ctx, id.Uint64())
	if err != nil {
		return nil, err
	}
	u := r.mapper.ToBO(po)
	if u == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

// FindByPhone 根据手机号查找用户
func (r *Repository) FindByPhone(ctx context.Context, phone meta.Phone) (*domain.User, error) {
	var po UserPO
	err := r.FindByField(ctx, &po, "phone", phone.String())
	if err != nil {
		return nil, err
	}
	u := r.mapper.ToBO(&po)
	if u == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

// Update 更新用户信息
func (r *Repository) Update(ctx context.Context, u *domain.User) error {
	po := r.mapper.ToPO(u)
	return r.UpdateAndSync(ctx, po, func(updated *UserPO) {
		id := meta.FromUint64(updated.ID.Uint64()) // ID 来自数据库，必定有效
		u.ID = id
	})
}
