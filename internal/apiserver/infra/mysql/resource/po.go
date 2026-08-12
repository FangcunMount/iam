package resource

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

// ResourcePO 资源持久化对象
type ResourcePO struct {
	base.AuditFields
	Key         string `gorm:"column:key;type:varchar(128);not null;uniqueIndex"`
	DisplayName string `gorm:"column:display_name;type:varchar(128)"`
	AppName     string `gorm:"column:app_name;type:varchar(32);index"`
	Domain      string `gorm:"column:domain;type:varchar(32);index"`
	Type        string `gorm:"column:type;type:varchar(32);index"`
	Actions     string `gorm:"column:actions;type:text"` // JSON array string
	ScopeKinds  string `gorm:"column:scope_kinds;type:text"`
	Description string `gorm:"column:description;type:varchar(512)"`
}

// TableName 指定表名
func (ResourcePO) TableName() string {
	return "authz_resources"
}

// BeforeCreate 在创建前设置信息
func (p *ResourcePO) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	id := meta.FromUint64(idutil.GetIntID()) // 新生成的 ID 必定有效
	createdBy := base.UserIDOrZero(tx.Statement.Context)
	updatedBy := createdBy
	deletedBy := meta.FromUint64(0)
	p.ID = id
	p.CreatedAt = now
	p.UpdatedAt = now
	p.CreatedBy = createdBy
	p.UpdatedBy = updatedBy
	p.DeletedBy = deletedBy
	p.Version = base.InitialVersion
	return nil
}

// BeforeUpdate 在更新前设置信息
func (r *ResourcePO) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now()

	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	r.UpdatedBy = updatedBy

	return nil
}
