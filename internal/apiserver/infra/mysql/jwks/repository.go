package jwks

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/infra/token/keyset"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"gorm.io/gorm"
)

// KeyRepository MySQL 实现，基于通用的 BaseRepository
type KeyRepository struct {
	mysql.BaseRepository[*KeyPO]
	mapper *Mapper
}

var _ domain.Repository = (*KeyRepository)(nil)

// NewKeyRepository 创建 KeyRepository 实例
func NewKeyRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*KeyPO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return errors.WithCode(code.ErrKeyAlreadyExists, "key with this kid already exists")
	}))

	return &KeyRepository{
		BaseRepository: base,
		mapper:         NewMapper(),
	}
}

// Save 保存密钥（创建）
func (r *KeyRepository) Save(ctx context.Context, key *domain.Key) error {
	po, err := r.mapper.ToKeyPO(key)
	if err != nil {
		return err
	}

	return r.CreateAndSync(ctx, po, func(updated *KeyPO) {
		key.CreatedAt = updated.CreatedAt
		key.UpdatedAt = updated.UpdatedAt
	})
}

// Update 更新密钥
func (r *KeyRepository) Update(ctx context.Context, key *domain.Key) error {
	po, err := r.mapper.ToKeyPO(key)
	if err != nil {
		return err
	}

	// 使用 kid 作为 WHERE 条件
	return r.WithContext(ctx).
		Model(&KeyPO{}).
		Where("kid = ?", key.Kid).
		Updates(po).Error
}

// Delete 删除密钥（物理删除）
func (r *KeyRepository) Delete(ctx context.Context, kid string) error {
	return r.WithContext(ctx).Where("kid = ?", kid).Delete(&KeyPO{}).Error
}

// FindByKid 根据 kid 查询密钥
func (r *KeyRepository) FindByKid(ctx context.Context, kid string) (*domain.Key, error) {
	var po KeyPO
	err := r.WithContext(ctx).Where("kid = ?", kid).First(&po).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return r.mapper.ToKeyEntity(&po)
}

// FindByStatus 根据状态查询密钥列表
func (r *KeyRepository) FindByStatus(ctx context.Context, status domain.KeyStatus) ([]*domain.Key, error) {
	var pos []*KeyPO
	err := r.WithContext(ctx).
		Where("status = ?", int8(status)).
		Order("created_at DESC").
		Find(&pos).Error
	if err != nil {
		return nil, err
	}

	return r.mapper.ToKeyEntities(pos)
}

// FindPublishable 查询可发布的密钥（Active + Grace）
func (r *KeyRepository) FindPublishable(ctx context.Context) ([]*domain.Key, error) {
	var pos []*KeyPO
	now := time.Now()

	err := r.WithContext(ctx).
		Where("status IN (?)", []int8{int8(domain.KeyActive), int8(domain.KeyGrace)}).
		Where("(not_before IS NULL OR not_before <= ?)", now).
		Where("(not_after IS NULL OR not_after > ?)", now).
		Order("kid ASC").
		Find(&pos).Error
	if err != nil {
		return nil, err
	}

	return r.mapper.ToKeyEntities(pos)
}

// FindExpired 查询已过期的密钥
func (r *KeyRepository) FindExpired(ctx context.Context) ([]*domain.Key, error) {
	var pos []*KeyPO
	now := time.Now()

	err := r.WithContext(ctx).
		Where("not_after IS NOT NULL AND not_after <= ?", now).
		Order("not_after ASC").
		Find(&pos).Error
	if err != nil {
		return nil, err
	}

	return r.mapper.ToKeyEntities(pos)
}

// FindAll 查询所有密钥（分页）
func (r *KeyRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.Key, int64, error) {
	var pos []*KeyPO
	var total int64

	// 查询总数
	if err := r.WithContext(ctx).Model(&KeyPO{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询数据
	query := r.WithContext(ctx).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&pos).Error
	if err != nil {
		return nil, 0, err
	}

	entities, err := r.mapper.ToKeyEntities(pos)
	if err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}

// CountByStatus 统计指定状态的密钥数量
func (r *KeyRepository) CountByStatus(ctx context.Context, status domain.KeyStatus) (int64, error) {
	var count int64
	err := r.WithContext(ctx).
		Model(&KeyPO{}).
		Where("status = ?", int8(status)).
		Count(&count).Error

	return count, err
}
