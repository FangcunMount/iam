package profilelink

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

// ProfileLinkPO 档案关系持久化对象
// 对应数据库表结构
type ProfileLinkPO struct {
	base.AuditFields
	UserID        meta.ID    `gorm:"column:user_id;type:bigint unsigned;not null;uniqueIndex:idx_user_profile_link,priority:1;comment:关系用户ID"`
	ProfileID     meta.ID    `gorm:"column:profile_id;type:bigint unsigned;not null;uniqueIndex:idx_user_profile_link,priority:2;comment:档案ID"`
	Type          string     `gorm:"column:type;type:varchar(32);not null;default:'relation';uniqueIndex:idx_user_profile_link,priority:3;index;comment:关系类型"`
	Relation      string     `gorm:"column:relation;type:varchar(16);not null;comment:档案关系"`
	SelfKey       *int64     `gorm:"column:self_key;type:bigint;uniqueIndex:uk_active_self_profile_link;comment:active self link user guard"`
	EstablishedAt time.Time  `gorm:"column:established_at;type:datetime;not null;comment:建立时间"`
	RevokedAt     *time.Time `gorm:"column:revoked_at;type:datetime;comment:撤销时间"`
}

// TableName 指定表名
func (ProfileLinkPO) TableName() string {
	return "profile_links"
}

// BeforeCreate 在创建前设置信息
func (g *ProfileLinkPO) BeforeCreate(tx *gorm.DB) error {
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
func (g *ProfileLinkPO) BeforeUpdate(tx *gorm.DB) error {
	g.UpdatedAt = time.Now()

	updatedBy := base.UserIDOrZero(tx.Statement.Context)
	g.UpdatedBy = updatedBy

	return nil
}
