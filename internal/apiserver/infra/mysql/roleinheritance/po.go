package roleinheritance

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

type InheritancePO struct {
	base.AuditFields
	TenantID        string     `gorm:"column:tenant_id;type:varchar(64);not null;uniqueIndex:uk_authz_role_inheritances_active,priority:1"`
	RoleID          uint64     `gorm:"column:role_id;type:bigint unsigned;not null;uniqueIndex:uk_authz_role_inheritances_active,priority:2"`
	InheritedRoleID uint64     `gorm:"column:inherited_role_id;type:bigint unsigned;not null;uniqueIndex:uk_authz_role_inheritances_active,priority:3"`
	GrantedBy       string     `gorm:"column:granted_by;type:varchar(64);not null"`
	GrantedAt       time.Time  `gorm:"column:granted_at;type:datetime;not null"`
	RevokedAt       *time.Time `gorm:"column:revoked_at;type:datetime"`
	ActiveGuard     *uint8     `gorm:"column:active_guard;type:tinyint GENERATED ALWAYS AS (CASE WHEN revoked_at IS NULL AND deleted_at IS NULL THEN 1 ELSE NULL END) STORED;->;uniqueIndex:uk_authz_role_inheritances_active,priority:4"`
}

func (InheritancePO) TableName() string { return "authz_role_inheritances" }

func (p *InheritancePO) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	p.ID = meta.FromUint64(idutil.GetIntID())
	p.CreatedAt = now
	p.UpdatedAt = now
	p.GrantedAt = now
	p.CreatedBy = base.UserIDOrZero(tx.Statement.Context)
	p.UpdatedBy = p.CreatedBy
	p.DeletedBy = meta.FromUint64(0)
	p.Version = base.InitialVersion
	return nil
}
