package rolebinding

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
	"gorm.io/gorm"
)

// BindingRepository Binding 仓储实现
type BindingRepository struct {
	mysql.BaseRepository[*BindingPO]
	mapper *Mapper
	db     *gorm.DB
}

var _ domain.Repository = (*BindingRepository)(nil)

// NewBindingRepository 创建 Binding 仓储
func NewBindingRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*BindingPO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrAssignmentAlreadyExists, "binding already exists")
	}))

	return &BindingRepository{
		BaseRepository: base,
		mapper:         NewMapper(),
		db:             db,
	}
}

// Create 创建新分配
func (r *BindingRepository) Create(ctx context.Context, a *domain.Binding) error {
	po := r.mapper.ToPO(a)

	return r.BaseRepository.CreateAndSync(ctx, po, func(updated *BindingPO) {
		a.ID = domain.BindingID(updated.ID)
	})
}

// FindByID 根据ID查找分配
func (r *BindingRepository) FindByID(ctx context.Context, id domain.BindingID) (*domain.Binding, error) {
	po, err := r.BaseRepository.FindByID(ctx, id.Uint64())
	if err != nil {
		return nil, fmt.Errorf("failed to find domain: %w", err)
	}

	bo := r.mapper.ToBO(po)
	if bo == nil {
		return nil, gorm.ErrRecordNotFound
	}

	return bo, nil
}

// ListBySubject 根据主体列出赋权
func (r *BindingRepository) ListBySubject(ctx context.Context, subjectType domain.SubjectType, subjectID, tenantID string) ([]*domain.Binding, error) {
	var pos []*BindingPO

	err := r.WithContext(ctx).Where("tenant_id = ? AND subject_type = ? AND subject_id = ?", tenantID, string(subjectType), subjectID).
		Find(&pos).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list domains by subject: %w", err)
	}

	bos := r.mapper.ToBOList(pos)

	return bos, nil
}

// ListByRole 根据角色列出赋权
func (r *BindingRepository) ListByRole(ctx context.Context, roleID uint64, tenantID string) ([]*domain.Binding, error) {
	var pos []*BindingPO

	err := r.WithContext(ctx).Where("tenant_id = ? AND role_id = ?", tenantID, roleID).
		Find(&pos).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list domains by role: %w", err)
	}

	bos := r.mapper.ToBOList(pos)

	return bos, nil
}

// Delete 删除分配
func (r *BindingRepository) Delete(ctx context.Context, id domain.BindingID) error {
	err := r.BaseRepository.DeleteByID(ctx, id.Uint64())
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}

// DeleteBySubjectAndRole 删除指定主体和角色的分配
func (r *BindingRepository) DeleteBySubjectAndRole(ctx context.Context, subjectType domain.SubjectType, subjectID string, roleID uint64, tenantID string) error {
	err := r.WithContext(ctx).Where("tenant_id = ? AND subject_type = ? AND subject_id = ? AND role_id = ?",
		tenantID, string(subjectType), subjectID, roleID).
		Delete(&BindingPO{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}
