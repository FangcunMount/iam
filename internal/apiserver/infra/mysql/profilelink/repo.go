package profilelink

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

// Repository 档案关系存储库实现
type Repository struct {
	mysql.BaseRepository[*ProfileLinkPO]
	mapper *ProfileLinkMapper
}

// NewRepository 创建档案关系存储库
func NewRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*ProfileLinkPO](db)
	// register a driver-aware translator that maps duplicate DB errors to the
	// profile-link-specific business error code.
	base.SetErrorTranslator(translateProfileLinkError)

	return &Repository{
		BaseRepository: base,
		mapper:         NewProfileLinkMapper(),
	}
}

func translateProfileLinkError(err error) error {
	return mysql.NewDuplicateToTranslator(func(e error) error {
		return errors.WithCode(code.ErrIdentityProfileLinkExists, "profile link already exists")
	})(err)
}

// Create 创建新的档案关系
func (r *Repository) Create(ctx context.Context, g *domain.ProfileLink) error {
	po := r.mapper.ToPO(g)
	return r.CreateAndSync(ctx, po, func(updated *ProfileLinkPO) {
		g.ID = updated.ID
		if updated.EstablishedAt.IsZero() {
			return
		}
		g.EstablishedAt = updated.EstablishedAt
	})
}

// FindByID 根据 ID 查找档案关系
func (r *Repository) FindByID(ctx context.Context, id meta.ID) (*domain.ProfileLink, error) {
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

// FindByProfileID 根据档案 ID 查找档案关系
func (r *Repository) FindByProfileID(ctx context.Context, id meta.ID) ([]*domain.ProfileLink, error) {
	return r.findByProfileID(ctx, id, false)
}

// FindByProfileIDIncludingRevoked 根据档案 ID 查找档案关系（包含已撤销）
func (r *Repository) FindByProfileIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*domain.ProfileLink, error) {
	return r.findByProfileID(ctx, id, true)
}

func (r *Repository) findByProfileID(ctx context.Context, id meta.ID, includeRevoked bool) ([]*domain.ProfileLink, error) {
	var pos []*ProfileLinkPO
	query := r.WithContext(ctx).Where("profile_id = ?", id.Uint64())
	if !includeRevoked {
		query = query.Where("revoked_at IS NULL")
	}
	if err := query.Find(&pos).Error; err != nil {
		return nil, err
	}

	return r.toDomainSlice(pos), nil
}

// FindByUserID 根据关系用户 ID 查找档案关系
func (r *Repository) FindByUserID(ctx context.Context, id meta.ID) ([]*domain.ProfileLink, error) {
	return r.findByUserID(ctx, id, false)
}

// FindActiveByUserIDAndType 根据关系用户 ID、关系类型查找有效 links。
func (r *Repository) FindActiveByUserIDAndType(ctx context.Context, userID meta.ID, typ domain.Type) ([]*domain.ProfileLink, error) {
	return r.findByUserIDAndType(ctx, userID, typ, false)
}

// FindByUserIDAndTypeIncludingRevoked 根据关系用户 ID、关系类型查找 links，包含已撤销。
func (r *Repository) FindByUserIDAndTypeIncludingRevoked(ctx context.Context, userID meta.ID, typ domain.Type) ([]*domain.ProfileLink, error) {
	return r.findByUserIDAndType(ctx, userID, typ, true)
}

func (r *Repository) findByUserIDAndType(ctx context.Context, userID meta.ID, typ domain.Type, includeRevoked bool) ([]*domain.ProfileLink, error) {
	var pos []*ProfileLinkPO

	query := r.WithContext(ctx).
		Where("user_id = ? AND type = ?", userID.Uint64(), string(typ))

	if !includeRevoked {
		query = query.Where("revoked_at IS NULL")
	}

	if err := query.Find(&pos).Error; err != nil {
		return nil, err
	}

	return r.toDomainSlice(pos), nil
}

// FindByUserIDIncludingRevoked 根据关系用户 ID 查找档案关系（包含已撤销）
func (r *Repository) FindByUserIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*domain.ProfileLink, error) {
	return r.findByUserID(ctx, id, true)
}

func (r *Repository) findByUserID(ctx context.Context, id meta.ID, includeRevoked bool) ([]*domain.ProfileLink, error) {
	var pos []*ProfileLinkPO
	query := r.WithContext(ctx).Where("user_id = ?", id.Uint64())
	if !includeRevoked {
		query = query.Where("revoked_at IS NULL")
	}
	if err := query.Find(&pos).Error; err != nil {
		return nil, err
	}

	return r.toDomainSlice(pos), nil
}

// FindByUserIDAndProfileID 根据关系用户 ID 和档案 ID 查找档案关系
func (r *Repository) FindByUserIDAndProfileID(ctx context.Context, userID meta.ID, profileID meta.ID) (*domain.ProfileLink, error) {
	return r.findByUserIDAndProfileID(ctx, userID, profileID, false)
}

// FindByUserIDAndProfileIDIncludingRevoked 根据关系用户 ID 和档案 ID 查找档案关系（包含已撤销）
func (r *Repository) FindByUserIDAndProfileIDIncludingRevoked(ctx context.Context, userID meta.ID, profileID meta.ID) (*domain.ProfileLink, error) {
	return r.findByUserIDAndProfileID(ctx, userID, profileID, true)
}

func (r *Repository) findByUserIDAndProfileID(ctx context.Context, userID meta.ID, profileID meta.ID, includeRevoked bool) (*domain.ProfileLink, error) {
	var po ProfileLinkPO
	query := r.WithContext(ctx).Where("user_id = ? AND profile_id = ?", userID.Uint64(), profileID.Uint64())
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

// IsLinked 检查是否为档案关系
func (r *Repository) IsLinked(ctx context.Context, userID meta.ID, profileID meta.ID) (bool, error) {
	var count int64
	if err := r.WithContext(ctx).Model(&ProfileLinkPO{}).
		Where("user_id = ? AND profile_id = ? AND revoked_at IS NULL", userID.Uint64(), profileID.Uint64()).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Update 更新档案关系
func (r *Repository) Update(ctx context.Context, g *domain.ProfileLink) error {
	po := r.mapper.ToPO(g)
	updates := map[string]interface{}{
		"user_id":        po.UserID,
		"profile_id":     po.ProfileID,
		"type":           po.Type,
		"relation":       po.Relation,
		"self_key":       po.SelfKey,
		"established_at": po.EstablishedAt,
		"revoked_at":     po.RevokedAt,
	}
	result := r.WithContext(ctx).Model(&ProfileLinkPO{}).Where("id = ?", po.ID.Uint64()).Updates(updates)
	if result.Error != nil {
		return translateProfileLinkError(result.Error)
	}
	g.ID = po.ID
	g.EstablishedAt = po.EstablishedAt
	g.RevokedAt = po.RevokedAt
	return nil
}

func (r *Repository) toDomainSlice(pos []*ProfileLinkPO) []*domain.ProfileLink {
	bos := r.mapper.ToBOs(pos)
	profileLinks := make([]*domain.ProfileLink, 0, len(bos))
	for _, bo := range bos {
		if bo == nil {
			continue
		}
		profileLinks = append(profileLinks, bo)
	}
	return profileLinks
}
