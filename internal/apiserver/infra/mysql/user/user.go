package user

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// UserPO 用户持久化对象
// 对应数据库表结构
type UserPO struct {
	base.AuditFields
	Name     string      `gorm:"column:name;type:varchar(64);not null;comment:用户名称"`
	Nickname string      `gorm:"column:nickname;type:varchar(64);comment:用户昵称"`
	Phone    meta.Phone  `gorm:"column:phone;type:varchar(20);index;comment:手机号（可为空）"`
	Email    meta.Email  `gorm:"column:email;type:varchar(100);not null;comment:邮箱"`
	IDCard   meta.IDCard `gorm:"column:id_card;type:varchar(20);uniqueIndex;comment:身份证号（可为空）"`
	Status   uint8       `gorm:"column:status;type:int;not null;default:1;comment:用户状态"`
}

// TableName 指定表名
func (UserPO) TableName() string {
	return "users"
}

// BeforeCreate 在创建前设置信息
func (p *UserPO) BeforeCreate(tx *gorm.DB) error {
	// 仅在 ID 未设置时生成新 ID
	if p.ID.Uint64() == 0 {
		newID := idutil.GetIntID()
		id := meta.FromUint64(newID) // 新生成的 ID 必定有效
		p.ID = id
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	createdBy := base.UserIDOrZero(tx.Statement.Context)
	updatedBy := createdBy
	deletedBy := meta.FromUint64(0)
	p.CreatedBy = createdBy
	p.UpdatedBy = updatedBy
	p.DeletedBy = deletedBy
	p.Version = base.InitialVersion

	return nil
} // BeforeUpdate 在更新前设置信息
func (u *UserPO) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now()
	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	u.UpdatedBy = updatedBy

	return nil
}
