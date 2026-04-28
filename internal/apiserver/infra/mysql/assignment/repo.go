package assignment

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"gorm.io/gorm"
)

// AssignmentRepository Assignment 仓储实现
type AssignmentRepository struct {
	mysql.BaseRepository[*AssignmentPO]
	mapper *Mapper
	db     *gorm.DB
}

var _ domain.Repository = (*AssignmentRepository)(nil)

// NewAssignmentRepository 创建 Assignment 仓储
func NewAssignmentRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*AssignmentPO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrAssignmentAlreadyExists, "assignment already exists")
	}))

	return &AssignmentRepository{
		BaseRepository: base,
		mapper:         NewMapper(),
		db:             db,
	}
}

// Create 创建新分配
func (r *AssignmentRepository) Create(ctx context.Context, a *domain.Assignment) error {
	po := r.mapper.ToPO(a)

	return r.BaseRepository.CreateAndSync(ctx, po, func(updated *AssignmentPO) {
		a.ID = domain.AssignmentID(updated.ID)
	})
}

// FindByID 根据ID查找分配
func (r *AssignmentRepository) FindByID(ctx context.Context, id domain.AssignmentID) (*domain.Assignment, error) {
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
func (r *AssignmentRepository) ListBySubject(ctx context.Context, subjectType domain.SubjectType, subjectID, tenantID string) ([]*domain.Assignment, error) {
	var pos []*AssignmentPO

	err := r.db.WithContext(ctx).Where("tenant_id = ? AND subject_type = ? AND subject_id = ?", tenantID, string(subjectType), subjectID).
		Find(&pos).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list domains by subject: %w", err)
	}

	bos := r.mapper.ToBOList(pos)

	return bos, nil
}

// ListByRole 根据角色列出赋权
func (r *AssignmentRepository) ListByRole(ctx context.Context, roleID uint64, tenantID string) ([]*domain.Assignment, error) {
	var pos []*AssignmentPO

	err := r.db.WithContext(ctx).Where("tenant_id = ? AND role_id = ?", tenantID, roleID).
		Find(&pos).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list domains by role: %w", err)
	}

	bos := r.mapper.ToBOList(pos)

	return bos, nil
}

// Delete 删除分配
func (r *AssignmentRepository) Delete(ctx context.Context, id domain.AssignmentID) error {
	err := r.BaseRepository.DeleteByID(ctx, id.Uint64())
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}

// DeleteBySubjectAndRole 删除指定主体和角色的分配
func (r *AssignmentRepository) DeleteBySubjectAndRole(ctx context.Context, subjectType domain.SubjectType, subjectID string, roleID uint64, tenantID string) error {
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND subject_type = ? AND subject_id = ? AND role_id = ?",
		tenantID, string(subjectType), subjectID, roleID).
		Delete(&AssignmentPO{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}
