package messagingcutover

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	migratedatabase "github.com/golang-migrate/migrate/v4/database"
	"strings"
	"time"
)

const ProtectionSchema = "iam-m3-additive-protection/v1"

// Protection deliberately excludes all existing business/AuthN tables. The only
// original object recorded is the real version journal, whose DDL must remain
// unchanged: restoration never executes a DDL string taken from this file.
type Protection struct {
	Schema             string   `json:"schema"`
	Scope              string   `json:"scope"`
	StoreID            string   `json:"store_id"`
	WriterEpoch        string   `json:"writer_epoch"`
	MigrationDigest    string   `json:"migration_digest"`
	Before             Snapshot `json:"before"`
	OriginalVersionDDL string   `json:"original_version_ddl"`
}

func CaptureProtection(ctx context.Context, db *sql.DB, store, epoch string) (Protection, error) {
	var p Protection
	if ctx == nil || db == nil {
		return p, ErrRefused
	}
	if !originalJournalShape(ctx, db) {
		return p, ErrRefused
	}
	s, err := Read(ctx, db)
	if err != nil {
		return p, err
	}
	plan, err := Build("schema", store, epoch, s)
	if err != nil || (s.Version != 40 && s.Version != 41) || len(s.Objects) != 0 || s.LegacyPending {
		return p, ErrRefused
	}
	var table, ddl string
	if db.QueryRowContext(ctx, "SHOW CREATE TABLE schema_migrations").Scan(&table, &ddl) != nil || table != "schema_migrations" || digest([]byte(ddl)) != s.VersionDDLHash {
		return p, ErrRefused
	}
	p = Protection{ProtectionSchema, protectionScope(s.Version), store, epoch, plan.MigrationDigest, s, ddl}
	return p, validateProtection(p)
}
func protectionScope(version uint) string {
	if version == 40 {
		return "eight-new-objects-and-original-version-journal"
	}
	return "seven-new-objects-and-original-version-journal"
}

func validateProtection(p Protection) error {
	if p.Schema != ProtectionSchema || p.Scope != protectionScope(p.Before.Version) || (p.Before.Version != 40 && p.Before.Version != 41) || len(p.Before.Objects) != 0 || p.Before.Binding != nil || p.Before.Selection != nil || p.Before.LegacyPending || len(p.OriginalVersionDDL) > 65536 || !strings.HasPrefix(p.OriginalVersionDDL, "CREATE TABLE `schema_migrations` (") || digest([]byte(p.OriginalVersionDDL)) != p.Before.VersionDDLHash {
		return ErrRefused
	}
	plan, err := Build("schema", p.StoreID, p.WriterEpoch, p.Before)
	if err != nil || plan.MigrationDigest != p.MigrationDigest {
		return ErrRefused
	}
	return nil
}
func EncodeProtection(p Protection) ([]byte, error) {
	if err := validateProtection(p); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(p)
	return append(raw, '\n'), err
}
func DecodeProtection(raw []byte, reviewed string) (Protection, error) {
	var p Protection
	if len(raw) > 131072 || digest(raw) != reviewed || json.Unmarshal(raw, &p) != nil {
		return p, ErrRefused
	}
	canonical, err := EncodeProtection(p)
	if err != nil || string(canonical) != string(raw) {
		return p, ErrRefused
	}
	return p, nil
}

// RestoreProtection is ONLY an initial pre-runtime scope restore. It never
// clears any request/execution/witness/operation/notice, even terminal records.
// Coordinator first-start eligibility and actual writer exclusion are external
// mandatory conditions, not proved by calling this function.
func RestoreProtection(ctx context.Context, p Protection, open func() (*sql.DB, error)) error {
	if ctx == nil || open == nil {
		return ErrRefused
	}
	if err := validateProtection(p); err != nil {
		return err
	}
	db, err := open()
	if err != nil {
		return ErrRefused
	}
	defer func() { _ = db.Close() }()
	conn, err := db.Conn(ctx)
	if err != nil {
		return ErrRefused
	}
	defer func() { _ = conn.Close() }()
	var acquired int
	lockName := "iam-m3-" + digest([]byte(p.Before.Database))[:48]
	if conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lockName).Scan(&acquired) != nil || acquired != 1 {
		return ErrRefused
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var n sql.NullInt64
		_ = conn.QueryRowContext(c, "SELECT RELEASE_LOCK(?)", lockName).Scan(&n)
	}()
	// Compete with the real golang-migrate lock too, including a still-running
	// migration that was not launched through this host helper.
	migrationLock, err := migratedatabase.GenerateAdvisoryLockId(p.Before.Database + ":schema_migrations")
	if err != nil || conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", migrationLock).Scan(&acquired) != nil || acquired != 1 {
		return ErrRefused
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var n sql.NullInt64
		_ = conn.QueryRowContext(c, "SELECT RELEASE_LOCK(?)", migrationLock).Scan(&n)
	}()
	var foreignKeys int
	if conn.QueryRowContext(ctx, "SELECT @@foreign_key_checks").Scan(&foreignKeys) != nil || foreignKeys != 1 {
		return ErrRefused
	}
	var database, uuid string
	if db.QueryRowContext(ctx, "SELECT DATABASE(),@@server_uuid").Scan(&database, &uuid) != nil || database != p.Before.Database || uuid != p.Before.ServerUUID {
		return ErrRefused
	}
	if !originalJournalShape(ctx, db) {
		return ErrRefused
	}
	ddl, err := ddlHash(ctx, db, "schema_migrations")
	if err != nil || ddl != p.Before.VersionDDLHash {
		return ErrRefused
	}
	versions, err := db.QueryContext(ctx, "SELECT version,dirty FROM schema_migrations LIMIT 2")
	if err != nil {
		return ErrRefused
	}
	var current uint
	var dirty bool
	if !versions.Next() || versions.Scan(&current, &dirty) != nil {
		_ = versions.Close()
		return ErrRefused
	}
	extra := versions.Next()
	rowErr := versions.Err()
	_ = versions.Close()
	if extra || rowErr != nil || current < p.Before.Version || current > 45 || (current == p.Before.Version && !dirty) {
		return ErrRefused
	}
	// A missing or out-of-range journal is never forced. A recorded in-range dirty value is
	// recoverable only via this reviewed original-scope restore, not schema apply.
	var legacyPending bool
	if db.QueryRowContext(ctx, "SELECT (EXISTS(SELECT 1 FROM rm_outbox WHERE BINARY state <> 'published' LIMIT 1) OR EXISTS(SELECT 1 FROM domain_event_outbox WHERE BINARY status <> 'published' LIMIT 1))").Scan(&legacyPending) != nil || legacyPending {
		return ErrRefused
	}
	present := []string{}
	restoreTables := append([]string(nil), tables...)
	if p.Before.Version == 40 {
		// Only schema40 protection proved this table did not exist. A schema41
		// protection must preserve its audit rows even if new failures arrived.
		restoreTables = append(restoreTables, "iam_message_failure_audit")
	}
	for _, name := range restoreTables {
		var n int
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND BINARY TABLE_NAME=BINARY ? AND TABLE_TYPE='BASE TABLE' AND ENGINE='InnoDB'", name).Scan(&n) != nil || n > 1 {
			return ErrRefused
		}
		var anyObject int
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND BINARY TABLE_NAME=BINARY ?", name).Scan(&anyObject) != nil || anyObject != n {
			return ErrRefused
		}
		if n == 0 {
			continue
		}
		var triggers int
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND BINARY EVENT_OBJECT_TABLE=BINARY ?", name).Scan(&triggers) != nil || triggers != 0 {
			return ErrRefused
		}
		switch name {
		case "rm_tx_binding":
			rows, err := db.QueryContext(ctx, "SELECT owner,store_id,writer_epoch,schema_version FROM rm_tx_binding LIMIT 2")
			if err != nil {
				return ErrRefused
			}
			if rows.Next() {
				var b Binding
				if rows.Scan(&b.Owner, &b.Store, &b.Epoch, &b.Version) != nil || b != (Binding{"iam.mysql", p.StoreID, p.WriterEpoch, 1}) {
					_ = rows.Close()
					return ErrRefused
				}
			}
			extra := rows.Next()
			rowErr := rows.Err()
			_ = rows.Close()
			if extra || rowErr != nil {
				return ErrRefused
			}
		case "iam_authz_message_protocol":
			rows, err := db.QueryContext(ctx, "SELECT owner,protocol_type,store_id,writer_epoch,route_revision,generation,schema_version FROM iam_authz_message_protocol LIMIT 2")
			if err != nil {
				return ErrRefused
			}
			if rows.Next() {
				var owner string
				var s Selection
				if rows.Scan(&owner, &s.Protocol, &s.Store, &s.Epoch, &s.Route, &s.Generation, &s.Version) != nil || owner != "iam.mysql" || (s != (Selection{Protocol: "outbox", Generation: 1, Version: 1}) && s != (Selection{"transaction", p.StoreID, p.WriterEpoch, "m3-1", 2, 1})) {
					_ = rows.Close()
					return ErrRefused
				}
			}
			extra := rows.Next()
			rowErr := rows.Err()
			_ = rows.Close()
			if extra || rowErr != nil {
				return ErrRefused
			}
		default:
			var nonempty bool
			if db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM `"+name+"` LIMIT 1)").Scan(&nonempty) != nil || nonempty {
				return ErrRefused
			}
		}
		present = append(present, "`"+name+"`")
	}
	// Foreign key checking remains enabled. Unexpected references cause a hard
	// failure; no FOREIGN_KEY_CHECKS override, business table restore or SQL import.
	if len(present) > 0 {
		if _, err = conn.ExecContext(ctx, "DROP TABLE "+strings.Join(present, ",")); err != nil {
			return ErrRefused
		}
	}
	// The original table itself was not changed. Restore its one original row by
	// CAS in one root, without dropping/recreating it or rewriting business data.
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return ErrRefused
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, "UPDATE schema_migrations SET version=?,dirty=FALSE WHERE version=? AND dirty=?", p.Before.Version, current, dirty)
	if err != nil {
		return ErrRefused
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return ErrRefused
	}
	if tx.Commit() != nil {
		return errors.New("IAM original journal restore unknown; keep operation closed and inspect exact scope")
	}
	after, err := Read(ctx, db)
	if err != nil {
		return ErrRefused
	}
	raw, _ := json.Marshal(after)
	original, _ := json.Marshal(p.Before)
	if string(raw) != string(original) {
		return ErrRefused
	}
	return nil
}

// The row contains exactly the two values that are protected/restored. Reject
// extra/generated/nullable columns or journal triggers instead of losing them.
func originalJournalShape(ctx context.Context, db *sql.DB) bool {
	var total, expected, triggers int
	var engine int
	if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND BINARY TABLE_NAME='schema_migrations' AND TABLE_TYPE='BASE TABLE' AND ENGINE='InnoDB'").Scan(&engine) != nil || engine != 1 {
		return false
	}
	if db.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM((BINARY COLUMN_NAME='version' AND DATA_TYPE='bigint' OR BINARY COLUMN_NAME='dirty' AND DATA_TYPE='tinyint') AND IS_NULLABLE='NO' AND EXTRA='' AND GENERATION_EXPRESSION=''),0) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND BINARY TABLE_NAME='schema_migrations'").Scan(&total, &expected) != nil || total != 2 || expected != 2 {
		return false
	}
	if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND BINARY EVENT_OBJECT_TABLE='schema_migrations'").Scan(&triggers) != nil || triggers != 0 {
		return false
	}
	return true
}
