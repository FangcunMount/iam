package loginidentity

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	base "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

type PO struct {
	base.AuditFields
	UserID           meta.ID    `gorm:"column:user_id;type:bigint unsigned;not null;index:idx_user_id"`
	Provider         string     `gorm:"column:provider;type:varchar(32);not null;uniqueIndex:uk_provider_realm_identifier,priority:1"`
	Realm            string     `gorm:"column:realm;type:varchar(128);not null;default:'';uniqueIndex:uk_provider_realm_identifier,priority:2"`
	Identifier       string     `gorm:"column:identifier;type:varchar(255);not null;uniqueIndex:uk_provider_realm_identifier,priority:3"`
	GlobalIdentifier *string    `gorm:"column:global_identifier;type:varchar(255);index:idx_global_identifier"`
	Status           string     `gorm:"column:status;type:varchar(32);not null"`
	VerifiedAt       *time.Time `gorm:"column:verified_at;type:datetime"`
	LinkedAt         time.Time  `gorm:"column:linked_at;type:datetime;not null"`
	Profile          []byte     `gorm:"column:profile_json;type:json"`
	Meta             []byte     `gorm:"column:meta_json;type:json"`
}

func (PO) TableName() string { return "auth_login_identities" }

func (p *PO) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if p.ID.IsZero() {
		p.ID = meta.FromUint64(idutil.GetIntID())
	}
	if p.LinkedAt.IsZero() {
		p.LinkedAt = now
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

func (p *PO) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	p.UpdatedBy = base.UserIDOrZero(tx.Statement.Context)
	return nil
}
