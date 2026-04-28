package guardianship

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// GuardianshipPO 监护关系持久化对象
// 对应数据库表结构
type GuardianshipPO struct {
	base.AuditFields
	UserID        meta.ID    `gorm:"column:user_id;type:bigint unsigned;not null;uniqueIndex:idx_user_child_ref,priority:1;comment:监护人ID"`
	ChildID       meta.ID    `gorm:"column:child_id;type:bigint unsigned;not null;uniqueIndex:idx_user_child_ref,priority:2;comment:儿童ID"`
	Relation      string     `gorm:"column:relation;type:varchar(16);not null;comment:监护关系"`
	EstablishedAt time.Time  `gorm:"column:established_at;type:datetime;not null;comment:建立时间"`
	RevokedAt     *time.Time `gorm:"column:revoked_at;type:datetime;comment:撤销时间"`
}

// TableName 指定表名
func (GuardianshipPO) TableName() string {
	return "guardianships"
}

// BeforeCreate 在创建前设置信息
func (g *GuardianshipPO) BeforeCreate(tx *gorm.DB) error {
	id := meta.FromUint64(idutil.GetIntID()) // 新生成的 ID 必定有效
	now := time.Now()
	createdBy := base.UserIDOrZero(tx.Statement.Context)
	updatedBy := createdBy
	deletedBy := meta.FromUint64(0)
	g.ID = id
	g.CreatedAt = now
	g.UpdatedAt = now
	g.CreatedBy = createdBy
	g.UpdatedBy = updatedBy
	g.DeletedBy = deletedBy

	return nil
}

// BeforeUpdate 在更新前设置信息
func (g *GuardianshipPO) BeforeUpdate(tx *gorm.DB) error {
	g.UpdatedAt = time.Now()

	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	g.UpdatedBy = updatedBy

	return nil
}
