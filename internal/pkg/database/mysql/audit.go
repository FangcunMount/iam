package mysql

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
)

// InitialVersion 默认的乐观锁版本号起点。
const InitialVersion uint32 = 1

// Syncable aggregates the behaviour required by persistence entities so that
// repositories can propagate auditing metadata back to domain models.
type Syncable interface {
	GetID() meta.ID
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	// DeletedAt is nullable; return pointer to allow distinguishing NULL
	GetDeletedAt() *time.Time
	GetCreatedBy() meta.ID
	GetUpdatedBy() meta.ID
	GetDeletedBy() meta.ID
	GetVersion() uint32
	SetID(meta.ID)
	SetCreatedAt(time.Time)
	SetUpdatedAt(time.Time)
	SetDeletedAt(*time.Time)
	SetCreatedBy(meta.ID)
	SetUpdatedBy(meta.ID)
	SetDeletedBy(meta.ID)
	SetVersion(uint32)
}

// AuditFields provides reusable columns for ID and audit timestamps.
type AuditFields struct {
	ID        meta.ID   `gorm:"primaryKey;type:bigint unsigned"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
	// Use pointer so GORM inserts NULL when no deleted time is set.
	DeletedAt *time.Time `gorm:"column:deleted_at;index"`
	CreatedBy meta.ID    `gorm:"column:created_by;type:bigint unsigned;default:0" json:"created_by"`
	UpdatedBy meta.ID    `gorm:"column:updated_by;type:bigint unsigned;default:0" json:"updated_by"`
	DeletedBy meta.ID    `gorm:"column:deleted_by;type:bigint unsigned;default:0" json:"deleted_by"`
	Version   uint32     `gorm:"column:version;type:int unsigned;not null;default:1;version" json:"version"`
}

func (a *AuditFields) GetID() meta.ID {
	return a.ID
}

func (a *AuditFields) GetCreatedAt() time.Time {
	return a.CreatedAt
}

func (a *AuditFields) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

func (a *AuditFields) GetDeletedAt() *time.Time {
	return a.DeletedAt
}

func (a *AuditFields) GetCreatedBy() meta.ID {
	return a.CreatedBy
}

func (a *AuditFields) GetUpdatedBy() meta.ID {
	return a.UpdatedBy
}

func (a *AuditFields) GetDeletedBy() meta.ID {
	return a.DeletedBy
}

func (a *AuditFields) GetVersion() uint32 {
	return a.Version
}

func (a *AuditFields) SetID(id meta.ID) {
	a.ID = id
}

func (a *AuditFields) SetCreatedAt(t time.Time) {
	a.CreatedAt = t
}

func (a *AuditFields) SetUpdatedAt(t time.Time) {
	a.UpdatedAt = t
}

func (a *AuditFields) SetDeletedAt(t *time.Time) {
	a.DeletedAt = t
}

func (a *AuditFields) SetCreatedBy(id meta.ID) {
	a.CreatedBy = id
}

func (a *AuditFields) SetUpdatedBy(id meta.ID) {
	a.UpdatedBy = id
}

func (a *AuditFields) SetDeletedBy(id meta.ID) {
	a.DeletedBy = id
}

func (a *AuditFields) SetVersion(v uint32) {
	a.Version = v
}

// UserIDFromContext extracts the authenticated user id from context when available.
func UserIDFromContext(ctx context.Context) (meta.ID, bool) {
	if ctx == nil {
		return 0, false
	}
	value := ctx.Value(authn.ContextKeyUserID)
	if value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case meta.ID:
		if v.IsZero() {
			return 0, false
		}
		return v, true
	case string:
		id, err := meta.ParseID(v)
		if err != nil || id.IsZero() {
			return 0, false
		}
		return id, true
	case uint64:
		if v == 0 {
			return 0, false
		}
		return meta.FromUint64(v), true
	case int64:
		if v <= 0 {
			return 0, false
		}
		return meta.FromUint64(uint64(v)), true
	case uint:
		if v == 0 {
			return 0, false
		}
		return meta.FromUint64(uint64(v)), true
	case int:
		if v <= 0 {
			return 0, false
		}
		return meta.FromUint64(uint64(v)), true
	default:
		return 0, false
	}
}

// UserIDOrZero returns the authenticated user id or 0 when not available.
func UserIDOrZero(ctx context.Context) meta.ID {
	if userID, ok := UserIDFromContext(ctx); ok {
		return userID
	}
	return meta.FromUint64(0)
}
