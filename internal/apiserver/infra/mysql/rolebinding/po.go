package rolebinding

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"gorm.io/gorm"
)

// BindingPO 赋权持久化对象
type BindingPO struct {
	base.AuditFields
	SubjectType string    `gorm:"column:subject_type;type:varchar(16);not null;index:idx_subject,priority:1"`
	SubjectID   string    `gorm:"column:subject_id;type:varchar(64);not null;index:idx_subject,priority:2"`
	RoleID      uint64    `gorm:"column:role_id;type:bigint unsigned;not null;index"`
	TenantID    string    `gorm:"column:tenant_id;type:varchar(64);not null;index"`
	GrantedBy   string    `gorm:"column:granted_by;type:varchar(64)"`
	GrantedAt   time.Time `gorm:"column:granted_at;type:datetime"`
}

// TableName 指定表名
func (BindingPO) TableName() string {
	return "authz_assignments"
}

// BeforeCreate 在创建前设置信息
func (p *BindingPO) BeforeCreate(tx *gorm.DB) error {
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
func (a *BindingPO) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now()

	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	a.UpdatedBy = updatedBy

	return nil
}
