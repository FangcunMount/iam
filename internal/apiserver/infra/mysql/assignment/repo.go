package assignment

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

// Repository Assignment 仓储实现。
type Repository struct {
	mysql.BaseRepository[*AssignmentPO]
	mapper *Mapper
	db     *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

// NewRepository 创建 Assignment 仓储。
func NewRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*AssignmentPO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrAssignmentAlreadyExists, "binding already exists")
	}))

	return &Repository{
		BaseRepository: base,
		mapper:         NewMapper(),
		db:             db,
	}
}

// Create 创建新分配
func (r *Repository) Create(ctx context.Context, a *domain.Assignment) error {
	po := r.mapper.ToPO(a)

	return r.BaseRepository.CreateAndSync(ctx, po, func(updated *AssignmentPO) {
		a.ID = domain.AssignmentID(updated.ID)
	})
}

// FindByID 根据ID查找分配
func (r *Repository) FindByID(ctx context.Context, id domain.AssignmentID) (*domain.Assignment, error) {
	po, err := r.BaseRepository.FindByID(ctx, id.Uint64())
	if err != nil {
		return nil, fmt.Errorf("failed to find domain: %w", err)
	}

	bo, err := r.mapper.ToBO(po)
	if err != nil {
		return nil, fmt.Errorf("failed to map domain: %w", err)
	}
	if bo == nil {
		return nil, gorm.ErrRecordNotFound
	}

	return bo, nil
}

// ListBySubject 根据主体列出赋权
func (r *Repository) ListBySubject(ctx context.Context, subjectType domain.SubjectType, subjectID meta.ID, tenantID string) ([]*domain.Assignment, error) {
	var pos []*AssignmentPO

	err := r.WithContext(ctx).Where("tenant_id = ? AND subject_type = ? AND subject_id = ?", tenantID, string(subjectType), subjectID.String()).
		Find(&pos).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list domains by subject: %w", err)
	}

	bos, err := r.mapper.ToBOList(pos)
	if err != nil {
		return nil, fmt.Errorf("failed to map domains by subject: %w", err)
	}

	return bos, nil
}

// ListByRole 根据角色列出赋权
func (r *Repository) ListByRole(ctx context.Context, roleID meta.ID, tenantID string) ([]*domain.Assignment, error) {
	var pos []*AssignmentPO

	err := r.WithContext(ctx).Where("tenant_id = ? AND role_id = ?", tenantID, roleID.Uint64()).
		Find(&pos).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list domains by role: %w", err)
	}

	bos, err := r.mapper.ToBOList(pos)
	if err != nil {
		return nil, fmt.Errorf("failed to map domains by role: %w", err)
	}

	return bos, nil
}

// Delete 删除分配
func (r *Repository) Delete(ctx context.Context, id domain.AssignmentID) error {
	err := r.BaseRepository.DeleteByID(ctx, id.Uint64())
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}

// DeleteBySubjectAndRole 删除指定主体和角色的分配
func (r *Repository) DeleteBySubjectAndRole(ctx context.Context, subjectType domain.SubjectType, subjectID meta.ID, roleID meta.ID, tenantID string) error {
	err := r.WithContext(ctx).Where("tenant_id = ? AND subject_type = ? AND subject_id = ? AND role_id = ?",
		tenantID, string(subjectType), subjectID.String(), roleID.Uint64()).
		Delete(&AssignmentPO{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}
