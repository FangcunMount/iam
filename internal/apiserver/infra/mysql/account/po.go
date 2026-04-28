package account

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// AccountPO 持久化对象，对应认证账号表。
type AccountPO struct {
	base.AuditFields
	UserID     meta.ID `gorm:"column:user_id;type:bigint unsigned;not null;index:idx_user_type,priority:1"`
	Type       string  `gorm:"column:type;type:varchar(32);not null;index:idx_user_type,priority:2;uniqueIndex:idx_type_app_external,priority:1"`
	AppID      *string `gorm:"column:app_id;type:varchar(64);uniqueIndex:idx_type_app_external,priority:2"`
	ExternalID string  `gorm:"column:external_id;type:varchar(128);not null;uniqueIndex:idx_type_app_external,priority:3"`
	// ScopedTenantID 运营账号租户作用域；非 opera 为 0。
	ScopedTenantID meta.ID `gorm:"column:scoped_tenant_id;type:bigint unsigned;not null;default:0;index"`
	UniqueID       *string `gorm:"column:unique_id;type:varchar(128);uniqueIndex:idx_unique_id"`
	Profile        []byte  `gorm:"column:profile;type:json"`
	Meta           []byte  `gorm:"column:meta;type:json"`
	Status         int8    `gorm:"column:status;type:tinyint;not null;default:1"`
}

// TableName 指定账号表名。
func (AccountPO) TableName() string {
	return "auth_accounts"
}

// BeforeCreate 在创建前设置信息。
func (p *AccountPO) BeforeCreate(tx *gorm.DB) error {
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

// BeforeUpdate 在更新前设置信息。
func (p *AccountPO) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	p.UpdatedBy = updatedBy
	return nil
}
