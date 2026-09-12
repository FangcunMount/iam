package policy

import (
	"context"
	"errors"
	"fmt"
	"math"

	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PolicyVersionRepository 保存唯一的全局策略版本，调用方负责授权事实与事件事务。
type PolicyVersionRepository struct {
	db     *gorm.DB
	mapper *Mapper
}

var _ domain.Repository = (*PolicyVersionRepository)(nil)

func NewPolicyVersionRepository(db *gorm.DB) domain.Repository {
	return &PolicyVersionRepository{db: db, mapper: NewMapper()}
}
func (r *PolicyVersionRepository) GetCurrent(ctx context.Context) (*domain.PolicyVersion, error) {
	var po PolicyVersionPO
	err := r.db.WithContext(ctx).First(&po, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.mapper.ToBO(&po), nil
}
func (r *PolicyVersionRepository) GetOrCreate(ctx context.Context) (*domain.PolicyVersion, error) {
	row := PolicyVersionPO{PolicyVersion: 1, ChangedBy: "system", Reason: "初始化全局策略版本"}
	row.ID = meta.ID(1)
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return nil, err
	}
	return r.GetCurrent(ctx)
}
func (r *PolicyVersionRepository) Increment(ctx context.Context, changedBy, reason string) (*domain.PolicyVersion, error) {
	if _, err := r.GetOrCreate(ctx); err != nil {
		return nil, err
	}
	var row PolicyVersionPO
	q := r.db.WithContext(ctx)
	if q.Dialector.Name() != "sqlite" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&row, 1).Error; err != nil {
		return nil, err
	}
	if row.PolicyVersion == math.MaxInt64 {
		return nil, fmt.Errorf("全局策略版本已达上限")
	}
	result := r.db.WithContext(ctx).Model(&PolicyVersionPO{}).Where("id = ? AND policy_version = ?", 1, row.PolicyVersion).Updates(map[string]any{"policy_version": row.PolicyVersion + 1, "changed_by": changedBy, "reason": reason})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("全局策略版本并发更新冲突")
	}
	return r.GetCurrent(ctx)
}

// GetCurrentForUpdate is called inside the assignment UOW after role/subject locks.
func (r *PolicyVersionRepository) GetCurrentForUpdate(ctx context.Context) (*domain.PolicyVersion, error) {
	var row PolicyVersionPO
	q := r.db.WithContext(ctx)
	if q.Dialector.Name() != "sqlite" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&row, 1).Error; err != nil {
		return nil, err
	}
	return r.mapper.ToBO(&row), nil
}
