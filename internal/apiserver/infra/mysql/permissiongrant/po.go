package permissiongrant

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

type GrantPO struct {
	base.AuditFields
	TenantID        string     `gorm:"column:tenant_id;type:varchar(64);not null;index:idx_authz_permission_grants_tenant_role,priority:1"`
	RoleID          uint64     `gorm:"column:role_id;type:bigint unsigned;not null;index:idx_authz_permission_grants_tenant_role,priority:2"`
	ResourceID      *uint64    `gorm:"column:resource_id;type:bigint unsigned;index"`
	ResourcePattern string     `gorm:"column:resource_pattern;type:varchar(128);not null"`
	Action          string     `gorm:"column:action;type:varchar(64);not null"`
	ConstraintSet   string     `gorm:"column:constraint_set;type:json;not null"`
	GrantKey        string     `gorm:"column:grant_key;type:char(64);not null;uniqueIndex:uk_authz_permission_grants_active,priority:1"`
	GrantedBy       string     `gorm:"column:granted_by;type:varchar(64);not null"`
	GrantedAt       time.Time  `gorm:"column:granted_at;type:datetime;not null"`
	RevokedAt       *time.Time `gorm:"column:revoked_at;type:datetime"`
	ActiveGuard     *uint8     `gorm:"column:active_guard;type:tinyint GENERATED ALWAYS AS (CASE WHEN revoked_at IS NULL AND deleted_at IS NULL THEN 1 ELSE NULL END) STORED;->;uniqueIndex:uk_authz_permission_grants_active,priority:2"`
}

func (GrantPO) TableName() string { return "authz_permission_grants" }

func (p *GrantPO) BeforeCreate(tx *gorm.DB) error {
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

func (p *GrantPO) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	p.UpdatedBy = base.UserIDOrZero(tx.Statement.Context)
	return nil
}
