package casbinrule

import (
	"context"

	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	dbmysql "github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type rulePO struct {
	ID    uint64  `gorm:"column:id;primaryKey;autoIncrement"`
	PType string  `gorm:"column:ptype"`
	V0    *string `gorm:"column:v0"`
	V1    *string `gorm:"column:v1"`
	V2    *string `gorm:"column:v2"`
	V3    *string `gorm:"column:v3"`
	V4    *string `gorm:"column:v4"`
	V5    *string `gorm:"column:v5"`
}

func (rulePO) TableName() string {
	return "casbin_rule"
}

type Repository struct {
	db *gorm.DB
}

var _ policyDomain.RuleStore = (*Repository)(nil)

func NewRepository(db *gorm.DB) policyDomain.RuleStore {
	return &Repository{db: db}
}

func (r *Repository) WithContext(ctx context.Context) *gorm.DB {
	if tx, ok := dbmysql.TxFromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *Repository) AddPolicy(ctx context.Context, rules ...policyDomain.PolicyRule) error {
	if len(rules) == 0 || r == nil || r.db == nil {
		return nil
	}
	rows := make([]rulePO, 0, len(rules))
	for _, rule := range rules {
		rows = append(rows, rulePO{
			PType: "p",
			V0:    stringPtr(rule.Sub),
			V1:    stringPtr(rule.Dom),
			V2:    stringPtr(rule.Obj),
			V3:    stringPtr(rule.Act),
		})
	}
	return r.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (r *Repository) RemovePolicy(ctx context.Context, rules ...policyDomain.PolicyRule) error {
	for _, rule := range rules {
		if err := r.WithContext(ctx).
			Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?", "p", rule.Sub, rule.Dom, rule.Obj, rule.Act).
			Delete(&rulePO{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) AddGroupingPolicy(ctx context.Context, rules ...policyDomain.GroupingRule) error {
	if len(rules) == 0 || r == nil || r.db == nil {
		return nil
	}
	rows := make([]rulePO, 0, len(rules))
	for _, rule := range rules {
		rows = append(rows, rulePO{
			PType: "g",
			V0:    stringPtr(rule.Sub),
			V1:    stringPtr(rule.Role),
			V2:    stringPtr(rule.Dom),
		})
	}
	return r.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (r *Repository) RemoveGroupingPolicy(ctx context.Context, rules ...policyDomain.GroupingRule) error {
	for _, rule := range rules {
		if err := r.WithContext(ctx).
			Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ?", "g", rule.Sub, rule.Role, rule.Dom).
			Delete(&rulePO{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func stringPtr(value string) *string {
	return &value
}
