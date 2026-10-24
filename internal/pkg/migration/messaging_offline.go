package migration

import (
	"context"
	"database/sql"
	"errors"
	"time"

	migratedatabase "github.com/golang-migrate/migrate/v4/database"
)

var ErrOfflineMessagingMigration = errors.New("offline additive IAM migration incomplete; preserve the original journal and remain stopped")

// RunMessagingOffline applies only the frozen additive 41..45 templates. Every
// SQL operation borrows the caller's real deadline. Generic RunTo uses upstream
// background contexts and cannot enforce the maintenance command's budget.
// No source/journal constructor is used and no missing journal is installed.
// The caller owns db. Each DDL may commit independently; dirty is persisted
// first and never cleared after an unknown/canceled operation.
func RunMessagingOffline(ctx context.Context, db *sql.DB, expected uint) (uint, error) {
	if ctx == nil || db == nil || expected < 40 || expected >= 45 {
		return 0, ErrOfflineMessagingMigration
	}
	if _, ok := ctx.Deadline(); !ok || ctx.Err() != nil {
		return 0, ErrOfflineMessagingMigration
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, ErrOfflineMessagingMigration
	}
	defer func() { _ = conn.Close() }()
	var database string
	if conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database) != nil || database == "" {
		return 0, ErrOfflineMessagingMigration
	}
	lockName, err := migratedatabase.GenerateAdvisoryLockId(database + ":schema_migrations")
	if err != nil {
		return 0, ErrOfflineMessagingMigration
	}
	var acquired int
	if conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lockName).Scan(&acquired) != nil || acquired != 1 {
		return 0, ErrOfflineMessagingMigration
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released sql.NullInt64
		_ = conn.QueryRowContext(cleanup, "SELECT RELEASE_LOCK(?)", lockName).Scan(&released)
	}()
	rows, err := conn.QueryContext(ctx, "SELECT version,dirty FROM schema_migrations LIMIT 2")
	if err != nil {
		return 0, ErrOfflineMessagingMigration
	}
	var current uint
	var dirty bool
	if !rows.Next() || rows.Scan(&current, &dirty) != nil {
		_ = rows.Close()
		return 0, ErrOfflineMessagingMigration
	}
	extra := rows.Next()
	rowErr := rows.Err()
	_ = rows.Close()
	if extra || rowErr != nil || dirty || current != expected {
		return current, ErrOfflineMessagingMigration
	}
	for current < 45 {
		next := current + 1
		name := messagingMigrationFile(next)
		raw, err := migrations.ReadFile("migrations/" + name)
		if err != nil || len(raw) == 0 {
			return current, ErrOfflineMessagingMigration
		}
		if err = casMessagingJournal(ctx, conn, current, false, next, true); err != nil {
			return current, ErrOfflineMessagingMigration
		}
		current = next // The database now records dirty at this version, even on cancellation.
		if _, err = conn.ExecContext(ctx, string(raw)); err != nil {
			return current, ErrOfflineMessagingMigration
		}
		if err = casMessagingJournal(ctx, conn, current, true, current, false); err != nil {
			return current, ErrOfflineMessagingMigration
		}
	}
	return current, nil
}
func casMessagingJournal(ctx context.Context, conn *sql.Conn, from uint, dirty bool, to uint, nextDirty bool) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, "UPDATE schema_migrations SET version=?,dirty=? WHERE version=? AND dirty=?", to, nextDirty, from, dirty)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return ErrOfflineMessagingMigration
	}
	return tx.Commit() // A lost COMMIT reply remains unknown, never a rollback proof.
}
func messagingMigrationFile(version uint) string {
	switch version {
	case 41:
		return "000041_message_failure_audit.up.sql"
	case 42:
		return "000042_transaction_messaging.up.sql"
	case 43:
		return "000043_authz_message_protocol.up.sql"
	case 44:
		return "000044_authz_business_operations.up.sql"
	case 45:
		return "000045_native_transaction_limit_notices.up.sql"
	default:
		return ""
	}
}
