package assignment

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

// AssignmentPO 赋权持久化对象。
type AssignmentPO struct {
	base.AuditFields
	SubjectType string `gorm:"column:subject_type;type:varchar(16);not null;index:idx_subject,priority:1;uniqueIndex:uk_authz_assignments_active,priority:1"`
	SubjectID   string `gorm:"column:subject_id;type:varchar(64);not null;index:idx_subject,priority:2;uniqueIndex:uk_authz_assignments_active,priority:2"`
	RoleID      uint64 `gorm:"column:role_id;type:bigint unsigned;not null;index;uniqueIndex:uk_authz_assignments_active,priority:3"`
	TenantID    string `gorm:"column:tenant_id;type:varchar(64);not null;index;uniqueIndex:uk_authz_assignments_active,priority:4"`
	// ActiveGuard is computed by the database. Active rows get 1; deleted rows
	// get NULL so MySQL's unique-key NULL semantics permit a later re-grant.
	ActiveGuard *uint8    `gorm:"column:active_guard;type:tinyint GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN 1 ELSE NULL END) STORED;->;uniqueIndex:uk_authz_assignments_active,priority:5"`
	GrantedBy   string    `gorm:"column:granted_by;type:varchar(64)"`
	GrantedAt   time.Time `gorm:"column:granted_at;type:datetime"`
}

// TableName 指定表名
func (AssignmentPO) TableName() string {
	return "authz_assignments"
}

// BeforeCreate 在创建前设置信息
func (p *AssignmentPO) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	id := meta.FromUint64(idutil.GetIntID()) // 新生成的 ID 必定有效
	createdBy := base.UserIDOrZero(tx.Statement.Context)
	updatedBy := createdBy
	deletedBy := meta.FromUint64(0)
	p.ID = id
	p.CreatedAt = now
	p.UpdatedAt = now
	p.GrantedAt = now
	p.CreatedBy = createdBy
	p.UpdatedBy = updatedBy
	p.DeletedBy = deletedBy
	p.Version = base.InitialVersion
	return nil
}

// BeforeUpdate 在更新前设置信息
func (a *AssignmentPO) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now()

	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	a.UpdatedBy = updatedBy

	return nil
}
