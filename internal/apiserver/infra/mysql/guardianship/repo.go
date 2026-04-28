package guardianship

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// Repository 监护关系存储库实现
type Repository struct {
	mysql.BaseRepository[*GuardianshipPO]
	mapper *GuardianshipMapper
}

// NewRepository 创建监护关系存储库
func NewRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*GuardianshipPO](db)
	// register a driver-aware translator that maps duplicate DB errors to the
	// guardianship-specific business error code.
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return errors.WithCode(code.ErrIdentityGuardianshipExists, "guardianship already exists")
	}))

	return &Repository{
		BaseRepository: base,
		mapper:         NewGuardianshipMapper(),
	}
}

// Create 创建新的监护关系
func (r *Repository) Create(ctx context.Context, g *domain.Guardianship) error {
	po := r.mapper.ToPO(g)
	return r.CreateAndSync(ctx, po, func(updated *GuardianshipPO) {
		g.ID = updated.ID
		if updated.EstablishedAt.IsZero() {
			return
		}
		g.EstablishedAt = updated.EstablishedAt
	})
}

// FindByID 根据 ID 查找监护关系
func (r *Repository) FindByID(ctx context.Context, id meta.ID) (*domain.Guardianship, error) {
	po, err := r.BaseRepository.FindByID(ctx, id.Uint64())
	if err != nil {
		return nil, err
	}
	g := r.mapper.ToBO(po)
	if g == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return g, nil
}

// FindByChildID 根据儿童 ID 查找监护关系
func (r *Repository) FindByChildID(ctx context.Context, id meta.ID) ([]*domain.Guardianship, error) {
	return r.findByChildID(ctx, id, false)
}

// FindByChildIDIncludingRevoked 根据儿童 ID 查找监护关系（包含已撤销）
func (r *Repository) FindByChildIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*domain.Guardianship, error) {
	return r.findByChildID(ctx, id, true)
}

func (r *Repository) findByChildID(ctx context.Context, id meta.ID, includeRevoked bool) ([]*domain.Guardianship, error) {
	var pos []*GuardianshipPO
	query := r.WithContext(ctx).Where("child_id = ?", id.Uint64())
	if !includeRevoked {
		query = query.Where("revoked_at IS NULL")
	}
	if err := query.Find(&pos).Error; err != nil {
		return nil, err
	}

	return r.toDomainSlice(pos), nil
}

// FindByUserID 根据监护人 ID 查找监护关系
func (r *Repository) FindByUserID(ctx context.Context, id meta.ID) ([]*domain.Guardianship, error) {
	return r.findByUserID(ctx, id, false)
}

// FindByUserIDIncludingRevoked 根据监护人 ID 查找监护关系（包含已撤销）
func (r *Repository) FindByUserIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*domain.Guardianship, error) {
	return r.findByUserID(ctx, id, true)
}

func (r *Repository) findByUserID(ctx context.Context, id meta.ID, includeRevoked bool) ([]*domain.Guardianship, error) {
	var pos []*GuardianshipPO
	query := r.WithContext(ctx).Where("user_id = ?", id.Uint64())
	if !includeRevoked {
		query = query.Where("revoked_at IS NULL")
	}
	if err := query.Find(&pos).Error; err != nil {
		return nil, err
	}

	return r.toDomainSlice(pos), nil
}

// FindByUserIDAndChildID 根据监护人 ID 和儿童 ID 查找监护关系
func (r *Repository) FindByUserIDAndChildID(ctx context.Context, userID meta.ID, childID meta.ID) (*domain.Guardianship, error) {
	return r.findByUserIDAndChildID(ctx, userID, childID, false)
}

// FindByUserIDAndChildIDIncludingRevoked 根据监护人 ID 和儿童 ID 查找监护关系（包含已撤销）
func (r *Repository) FindByUserIDAndChildIDIncludingRevoked(ctx context.Context, userID meta.ID, childID meta.ID) (*domain.Guardianship, error) {
	return r.findByUserIDAndChildID(ctx, userID, childID, true)
}

func (r *Repository) findByUserIDAndChildID(ctx context.Context, userID meta.ID, childID meta.ID, includeRevoked bool) (*domain.Guardianship, error) {
	var po GuardianshipPO
	query := r.WithContext(ctx).Where("user_id = ? AND child_id = ?", userID.Uint64(), childID.Uint64())
	if !includeRevoked {
		query = query.Where("revoked_at IS NULL")
	}
	if err := query.First(&po).Error; err != nil {
		return nil, err
	}

	g := r.mapper.ToBO(&po)
	if g == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return g, nil
}

// IsGuardian 检查是否为监护关系
func (r *Repository) IsGuardian(ctx context.Context, userID meta.ID, childID meta.ID) (bool, error) {
	var count int64
	if err := r.WithContext(ctx).Model(&GuardianshipPO{}).
		Where("user_id = ? AND child_id = ? AND revoked_at IS NULL", userID.Uint64(), childID.Uint64()).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Update 更新监护关系// Update 更新监护关系
func (r *Repository) Update(ctx context.Context, g *domain.Guardianship) error {
	po := r.mapper.ToPO(g)
	return r.UpdateAndSync(ctx, po, func(updated *GuardianshipPO) {
		g.ID = updated.ID
		g.EstablishedAt = updated.EstablishedAt
		g.RevokedAt = updated.RevokedAt
	})
}

func (r *Repository) toDomainSlice(pos []*GuardianshipPO) []*domain.Guardianship {
	bos := r.mapper.ToBOs(pos)
	guardianships := make([]*domain.Guardianship, 0, len(bos))
	for _, bo := range bos {
		if bo == nil {
			continue
		}
		guardianships = append(guardianships, bo)
	}
	return guardianships
}
