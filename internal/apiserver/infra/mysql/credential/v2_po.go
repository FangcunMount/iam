package credential

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"gorm.io/gorm"
)

type V2PO struct {
	base.AuditFields
	LoginIdentityID meta.ID    `gorm:"column:login_identity_id;type:bigint unsigned;not null;index:idx_login_identity_id;uniqueIndex:uk_identity_type,priority:1"`
	Type            string     `gorm:"column:type;type:varchar(32);not null;uniqueIndex:uk_identity_type,priority:2"`
	Material        []byte     `gorm:"column:material;type:varbinary(4096)"`
	Algo            *string    `gorm:"column:algo;type:varchar(64)"`
	Params          []byte     `gorm:"column:params_json;type:json"`
	Status          string     `gorm:"column:status;type:varchar(32);not null"`
	FailedAttempts  int        `gorm:"column:failed_attempts;type:int;not null;default:0"`
	LockedUntil     *time.Time `gorm:"column:locked_until;type:datetime"`
	LastSuccessAt   *time.Time `gorm:"column:last_success_at;type:datetime"`
	LastFailureAt   *time.Time `gorm:"column:last_failure_at;type:datetime"`
}

func (V2PO) TableName() string { return "auth_credentials" }

func (p *V2PO) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if p.ID.IsZero() {
		p.ID = meta.FromUint64(idutil.GetIntID())
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	createdBy := base.UserIDOrZero(tx.Statement.Context)
	p.CreatedBy = createdBy
	p.UpdatedBy = createdBy
	p.DeletedBy = meta.ZeroID
	if p.Version == 0 {
		p.Version = base.InitialVersion
	}
	return nil
}

func (p *V2PO) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	p.UpdatedBy = base.UserIDOrZero(tx.Statement.Context)
	return nil
}
