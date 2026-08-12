package profile

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

// ProfilePO 档案持久化对象
// 对应数据库表结构
type ProfilePO struct {
	base.AuditFields
	Name string `gorm:"column:name;type:varchar(64);not null;index:idx_name_gender_birthday,priority:1;comment:档案姓名"`
	// IDCard 是可空的；使用指针以便将空值写入 NULL，避免唯一索引对空字符串的冲突
	IDCard   *meta.IDCard `gorm:"column:id_card;type:varchar(20);uniqueIndex;comment:身份证号码"`
	Gender   uint8        `gorm:"column:gender;type:tinyint;not null;default:0;index:idx_name_gender_birthday,priority:2;comment:性别"`
	Birthday string       `gorm:"column:birthday;type:varchar(10);index:idx_name_gender_birthday,priority:3;comment:出生日期"`
}

// TableName 指定表名
func (ProfilePO) TableName() string {
	return "profiles"
}

// BeforeCreate 在创建前设置信息
func (p *ProfilePO) BeforeCreate(tx *gorm.DB) error {
	id := meta.FromUint64(idutil.GetIntID()) // 新生成的 ID 必定有效
	createdBy := base.UserIDOrZero(tx.Statement.Context)
	updatedBy := createdBy
	deletedBy := meta.FromUint64(0)
	p.ID = id
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	p.CreatedBy = createdBy
	p.UpdatedBy = updatedBy
	p.DeletedBy = deletedBy
	p.Version = base.InitialVersion

	return nil
}

// BeforeUpdate 在更新前设置信息
func (p *ProfilePO) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	p.UpdatedBy = updatedBy

	return nil
}
