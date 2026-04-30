package resource

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"gorm.io/gorm"
)

// ResourceRepository Resource 仓储实现
type ResourceRepository struct {
	mysql.BaseRepository[*ResourcePO]
	mapper *Mapper
	db     *gorm.DB
}

var _ domain.Repository = (*ResourceRepository)(nil)

// NewResourceRepository 创建 Resource 仓储
func NewResourceRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*ResourcePO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrResourceAlreadyExists, "resource already exists")
	}))

	return &ResourceRepository{
		BaseRepository: base,
		mapper:         NewMapper(),
		db:             db,
	}
}

// Create 创建新资源
func (r *ResourceRepository) Create(ctx context.Context, res *domain.Resource) error {
	po := r.mapper.ToPO(res)

	return r.BaseRepository.CreateAndSync(ctx, po, func(updated *ResourcePO) {
		res.ID = domain.NewResourceID(updated.ID.Uint64())
	})
}

// Update 更新资源
func (r *ResourceRepository) Update(ctx context.Context, res *domain.Resource) error {
	po := r.mapper.ToPO(res)

	return r.BaseRepository.UpdateAndSync(ctx, po, func(updated *ResourcePO) {
		// Sync if needed
	})
}

// FindByID 根据ID查找资源
func (r *ResourceRepository) FindByID(ctx context.Context, id domain.ResourceID) (*domain.Resource, error) {
	po, err := r.BaseRepository.FindByID(ctx, id.Uint64())
	if err != nil {
		return nil, fmt.Errorf("failed to find resource: %w", err)
	}

	bo := r.mapper.ToBO(po)
	if bo == nil {
		return nil, gorm.ErrRecordNotFound
	}

	return bo, nil
}

// FindByKey 根据资源键查找
func (r *ResourceRepository) FindByKey(ctx context.Context, key string) (*domain.Resource, error) {
	var po ResourcePO

	// `key` is a reserved word in MySQL; quote the column name to avoid syntax errors
	err := r.WithContext(ctx).Where("`key` = ?", key).First(&po).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find resource by key: %w", err)
	}

	bo := r.mapper.ToBO(&po)
	return bo, nil
}

// ListByApp 列出应用的资源
func (r *ResourceRepository) ListByApp(ctx context.Context, appName string, offset, limit int) ([]*domain.Resource, int64, error) {
	var pos []*ResourcePO
	var total int64

	// 统计总数
	if err := r.WithContext(ctx).Model(&ResourcePO{}).Where("app_name = ?", appName).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count resources: %w", err)
	}

	// 查询列表
	err := r.WithContext(ctx).
		Where("app_name = ?", appName).
		Order("`key` ASC").
		Offset(offset).
		Limit(limit).
		Find(&pos).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list resources: %w", err)
	}

	bos := r.mapper.ToBOList(pos)

	return bos, total, nil
}

// ListByDomain 列出业务域的资源
func (r *ResourceRepository) ListByDomain(ctx context.Context, domain string, offset, limit int) ([]*domain.Resource, int64, error) {
	var pos []*ResourcePO
	var total int64

	// 统计总数
	if err := r.WithContext(ctx).Model(&ResourcePO{}).Where("domain = ?", domain).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count resources: %w", err)
	}

	// 查询列表
	err := r.WithContext(ctx).
		Where("domain = ?", domain).
		Order("`key` ASC").
		Offset(offset).
		Limit(limit).
		Find(&pos).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list resources: %w", err)
	}

	bos := r.mapper.ToBOList(pos)

	return bos, total, nil
}

// List 列出资源（可按 app/domain/type 过滤）
func (r *ResourceRepository) List(ctx context.Context, query domain.ResourceFilter) ([]*domain.Resource, int64, error) {
	var pos []*ResourcePO
	var total int64

	db := r.WithContext(ctx).Model(&ResourcePO{})
	if query.AppName != "" {
		db = db.Where("app_name = ?", query.AppName)
	}
	if query.Domain != "" {
		db = db.Where("domain = ?", query.Domain)
	}
	if query.Type != "" {
		db = db.Where("`type` = ?", query.Type)
	}

	// 统计总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count resources: %w", err)
	}

	// 查询列表
	err := db.
		Order("`key` ASC").
		Offset(query.Offset).
		Limit(query.Limit).
		Find(&pos).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list resources: %w", err)
	}

	bos := r.mapper.ToBOList(pos)

	return bos, total, nil
}

// ValidateAction 校验动作是否在资源的允许列表中
func (r *ResourceRepository) ValidateAction(ctx context.Context, resourceKey, action string) (bool, error) {
	res, err := r.FindByKey(ctx, resourceKey)
	if err != nil {
		return false, fmt.Errorf("failed to find resource: %w", err)
	}

	if res == nil {
		return false, fmt.Errorf("resource not found: %s", resourceKey)
	}

	return res.HasAction(action), nil
}

// Delete 删除资源（软删除）
func (r *ResourceRepository) Delete(ctx context.Context, id domain.ResourceID) error {
	err := r.BaseRepository.DeleteByID(ctx, id.Uint64())
	if err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}

	return nil
}
