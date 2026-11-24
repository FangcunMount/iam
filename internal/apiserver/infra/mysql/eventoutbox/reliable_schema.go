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
	return readOutboxSnapshot(ctx, db, func(tx *gorm.DB) error {
		table, err := standardReadTable(tx)
		if err != nil {
			return err
		}
		if table != standardOutboxTable {
			return errors.New("historical standard Outbox has no SDK write ownership")
		}
		return checkReliableSchema(tx, table)
	})
}

func checkReliableSchema(db *gorm.DB, table string) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "mysql" {
		return errors.New("reliable messaging requires MySQL")
	}
	var versions []struct {
		Version uint64
		Dirty   bool
	}
	if err := db.Raw("SELECT version, dirty FROM schema_migrations").Scan(&versions).Error; err != nil {
		return err
	}
	if len(versions) != 1 || versions[0].Dirty || versions[0].Version < 39 {
		return errors.New("reliable messaging requires clean migration 39 or later")
	}
	// Resolve every required column even on an empty table. Exact column/index
	// definitions are owned by migrations 38/39 and their real MySQL tests.
	if err := db.Exec(`SELECT id, producer, message_id, destination, event_type, schema_version, scope,
        content_type, occurred_at, payload, fingerprint, state, next_attempt_at,
        claim_token, lease_until, version, attempt_count, last_error_code,
        transport_confirmed_at, created_at FROM ` + table + ` LIMIT 0`).Error; err != nil {
		return err
	}
	return db.Exec("SELECT failure_count, updated_at FROM " + table + " LIMIT 0").Error
}
