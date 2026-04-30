package casbinrule

import (
	"context"

	appuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	authzDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz"
	casbinfacts "github.com/FangcunMount/iam/internal/apiserver/infra/casbin"
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

var _ appuow.AuthorizationFactStore = (*Repository)(nil)

func NewRepository(db *gorm.DB) appuow.AuthorizationFactStore {
	return &Repository{db: db}
}

func (r *Repository) WithContext(ctx context.Context) *gorm.DB {
	if tx, ok := dbmysql.TxFromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *Repository) AddPermission(ctx context.Context, permission authzDomain.Permission) error {
	if r == nil || r.db == nil {
		return nil
	}
	rule := casbinfacts.PolicyRuleFromPermission(permission)
	row := rulePO{
		PType: "p",
		V0:    stringPtr(rule.Sub),
		V1:    stringPtr(rule.Dom),
		V2:    stringPtr(rule.Obj),
		V3:    stringPtr(rule.Act),
		V4:    stringPtr(rule.Scope),
	}
	return r.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (r *Repository) RemovePermission(ctx context.Context, permission authzDomain.Permission) error {
	rule := casbinfacts.PolicyRuleFromPermission(permission)
	db := r.WithContext(ctx).
		Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?", "p", rule.Sub, rule.Dom, rule.Obj, rule.Act).
		Where(scopeWhere(rule.Scope), scopeWhereArgs(rule.Scope)...)
	return db.Delete(&rulePO{}).Error
}

func (r *Repository) AddRoleBinding(ctx context.Context, binding authzDomain.RoleBinding) error {
	if r == nil || r.db == nil {
		return nil
	}
	rule := casbinfacts.GroupingRuleFromRoleBinding(binding)
	row := rulePO{
		PType: "g",
		V0:    stringPtr(rule.Sub),
		V1:    stringPtr(rule.Role),
		V2:    stringPtr(rule.Dom),
	}
	return r.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (r *Repository) RemoveRoleBinding(ctx context.Context, binding authzDomain.RoleBinding) error {
	rule := casbinfacts.GroupingRuleFromRoleBinding(binding)
	return r.WithContext(ctx).
		Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ?", "g", rule.Sub, rule.Role, rule.Dom).
		Delete(&rulePO{}).Error
}

func stringPtr(value string) *string {
	return &value
}

func scopeWhere(scope string) string {
	if casbinfacts.ScopeFromKey(scope).String() == casbinfacts.ScopeFromKey("").String() {
		return "(v4 = ? OR v4 IS NULL OR v4 = '')"
	}
	return "v4 = ?"
}

func scopeWhereArgs(scope string) []interface{} {
	return []interface{}{casbinfacts.ScopeKey(casbinfacts.ScopeFromKey(scope))}
}
