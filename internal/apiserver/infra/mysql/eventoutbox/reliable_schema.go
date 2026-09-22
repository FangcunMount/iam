package eventoutbox

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// CheckReliableSchema is explicit startup I/O; constructors never install DDL.
// This checks migration readiness, not the operational exclusive-writer handoff.
func CheckReliableSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "mysql" {
		return errors.New("reliable messaging requires MySQL")
	}
	var versions []struct {
		Version uint64
		Dirty   bool
	}
	if err := db.WithContext(ctx).Raw("SELECT version, dirty FROM schema_migrations").Scan(&versions).Error; err != nil {
		return err
	}
	if len(versions) != 1 || versions[0].Dirty || versions[0].Version < 38 {
		return errors.New("reliable messaging requires clean migration 38 or later")
	}
	// Resolve every required column even on an empty table. Exact column/index
	// definitions are owned by migration 38 and its real MySQL contract tests.
	return db.WithContext(ctx).Exec(`SELECT rm_claim_token, rm_claim_version, rm_claim_count,
		rm_lease_until, rm_fingerprint FROM domain_event_outbox LIMIT 0`).Error
}
