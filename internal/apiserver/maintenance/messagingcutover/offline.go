// Package messagingcutover prepares additive IAM M3 schema and explicit writer
// selection without starting a business runtime, SDK, Redis or MQ client.
package messagingcutover

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/FangcunMount/iam/v5/internal/pkg/migration"
)

const Schema = "iam-m3-offline/v1"

var ErrRefused = errors.New("IAM M3 offline state is unavailable, changed or unsafe")
var identifier = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
var tables = []string{"rm_tx_binding", "rm_tx_request", "rm_tx_execution", "rm_tx_witness", "iam_authz_message_protocol", "iam_authz_operations", "messaging_native_limit_notices"}

type Object struct {
	Name     string `json:"name"`
	DDLHash  string `json:"ddl_sha256"`
	Nonempty bool   `json:"nonempty"`
}
type Selection struct {
	Protocol   string `json:"protocol"`
	Store      string `json:"store"`
	Epoch      string `json:"epoch"`
	Route      string `json:"route"`
	Generation int64  `json:"generation"`
	Version    int    `json:"version"`
}
type Binding struct {
	Owner   string `json:"owner"`
	Store   string `json:"store"`
	Epoch   string `json:"epoch"`
	Version int    `json:"version"`
}
type Snapshot struct {
	Database       string     `json:"database"`
	ServerUUID     string     `json:"server_uuid"`
	Version        uint       `json:"version"`
	VersionDDLHash string     `json:"version_ddl_sha256"`
	Objects        []Object   `json:"objects"`
	Binding        *Binding   `json:"binding"`
	Selection      *Selection `json:"selection"`
	LegacyPending  bool       `json:"legacy_pending"`
}
type Plan struct {
	Schema          string   `json:"schema"`
	Action          string   `json:"action"`
	StoreID         string   `json:"store_id"`
	WriterEpoch     string   `json:"writer_epoch"`
	MigrationDigest string   `json:"migration_digest"`
	Before          Snapshot `json:"before"`
}

func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }

// Read uses only SELECT/SHOW. In particular, no migrate driver is constructed;
// its constructor can install schema_migrations even during a nominal preview.
func Read(ctx context.Context, db *sql.DB) (Snapshot, error) {
	var out Snapshot
	if ctx == nil || db == nil {
		return out, ErrRefused
	}
	if db.QueryRowContext(ctx, "SELECT DATABASE(),@@server_uuid").Scan(&out.Database, &out.ServerUUID) != nil || out.Database == "" || out.ServerUUID == "" {
		return out, ErrRefused
	}
	rows, err := db.QueryContext(ctx, "SELECT version,dirty FROM schema_migrations LIMIT 2")
	if err != nil {
		return out, ErrRefused
	}
	var dirty bool
	if !rows.Next() || rows.Scan(&out.Version, &dirty) != nil {
		_ = rows.Close()
		return out, ErrRefused
	}
	extra := rows.Next()
	scanErr := rows.Err()
	_ = rows.Close()
	if extra || scanErr != nil || dirty || out.Version < 40 || out.Version > 45 {
		return out, ErrRefused
	}
	// Schema40 has not installed the provider-aware audit table. Claiming its
	// ownership is safe only after proving that the exact name is absent.
	if out.Version == 40 {
		var auditObjects int
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND BINARY TABLE_NAME='iam_message_failure_audit'").Scan(&auditObjects) != nil || auditObjects != 0 {
			return out, ErrRefused
		}
	}
	ddl, err := ddlHash(ctx, db, "schema_migrations")
	if err != nil {
		return out, err
	}
	out.VersionDDLHash = ddl
	for i, name := range tables {
		minimum := uint(42)
		if i == 4 {
			minimum = 43
		}
		if i == 5 {
			minimum = 44
		}
		if i == 6 {
			minimum = 45
		}
		var n int
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND BINARY TABLE_NAME=BINARY ?", name).Scan(&n) != nil || n > 1 {
			return out, ErrRefused
		}
		if (n == 1) != (out.Version >= minimum) {
			return out, ErrRefused
		}
		if n == 0 {
			continue
		}
		hash, err := ddlHash(ctx, db, name)
		if err != nil {
			return out, err
		}
		var nonempty bool
		if db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM `"+name+"` LIMIT 1)").Scan(&nonempty) != nil {
			return out, ErrRefused
		}
		out.Objects = append(out.Objects, Object{name, hash, nonempty})
	}
	if out.Version >= 42 {
		values, err := db.QueryContext(ctx, "SELECT owner,store_id,writer_epoch,schema_version FROM rm_tx_binding LIMIT 2")
		if err != nil {
			return out, ErrRefused
		}
		if values.Next() {
			var b Binding
			if values.Scan(&b.Owner, &b.Store, &b.Epoch, &b.Version) != nil {
				_ = values.Close()
				return out, ErrRefused
			}
			out.Binding = &b
		}
		extra := values.Next()
		scanErr := values.Err()
		_ = values.Close()
		if extra || scanErr != nil {
			return out, ErrRefused
		}
	}
	if out.Version >= 43 {
		var owner string
		var s Selection
		rows, err := db.QueryContext(ctx, "SELECT owner,protocol_type,store_id,writer_epoch,route_revision,generation,schema_version FROM iam_authz_message_protocol LIMIT 2")
		if err != nil {
			return out, ErrRefused
		}
		if !rows.Next() || rows.Scan(&owner, &s.Protocol, &s.Store, &s.Epoch, &s.Route, &s.Generation, &s.Version) != nil {
			_ = rows.Close()
			return out, ErrRefused
		}
		extra := rows.Next()
		scanErr := rows.Err()
		_ = rows.Close()
		if extra || scanErr != nil || owner != "iam.mysql" {
			return out, ErrRefused
		}
		out.Selection = &s
	}
	// No removal or reset of legacy terminal rows. Pending/claimed/unknown states
	// block selection; the independent NSQ/Outbox responsibility proof is still required.
	if db.QueryRowContext(ctx, "SELECT (EXISTS(SELECT 1 FROM rm_outbox WHERE BINARY state <> 'published' LIMIT 1) OR EXISTS(SELECT 1 FROM domain_event_outbox WHERE BINARY status <> 'published' LIMIT 1))").Scan(&out.LegacyPending) != nil {
		return out, ErrRefused
	}
	return out, nil
}
func ddlHash(ctx context.Context, db *sql.DB, name string) (string, error) {
	var table, ddl string
	if db.QueryRowContext(ctx, "SHOW CREATE TABLE `"+name+"`").Scan(&table, &ddl) != nil || table != name {
		return "", ErrRefused
	}
	return digest([]byte(ddl)), nil
}
func Build(action, store, epoch string, snapshot Snapshot) (Plan, error) {
	p := Plan{Schema: Schema, Action: action, StoreID: store, WriterEpoch: epoch, MigrationDigest: migration.MessagingMigrationDigest(), Before: snapshot}
	return p, Validate(p)
}
func Validate(p Plan) error {
	if p.Schema != Schema || !identifier.MatchString(p.StoreID) || !identifier.MatchString(p.WriterEpoch) || p.MigrationDigest != migration.MessagingMigrationDigest() || p.Before.Database == "" || p.Before.ServerUUID == "" || p.Before.Version < 40 || p.Before.Version > 45 {
		return ErrRefused
	}
	if p.Action != "schema" && p.Action != "select" {
		return ErrRefused
	}
	// This entry is an initial installation only, never a protocol reset/recovery.
	for _, obj := range p.Before.Objects {
		if obj.Nonempty && obj.Name != "iam_authz_message_protocol" {
			return ErrRefused
		}
	}
	if p.Before.Binding != nil {
		return ErrRefused
	}
	if p.Before.Selection != nil && *p.Before.Selection != (Selection{Protocol: "outbox", Generation: 1, Version: 1}) {
		return ErrRefused
	}
	if p.Action == "schema" && p.Before.Version >= 45 {
		return ErrRefused
	}
	if p.Action == "select" && (p.Before.Version != 45 || p.Before.Selection == nil || p.Before.LegacyPending) {
		return ErrRefused
	}
	return nil
}

// Apply owns all connections opened by open. The independent lock connection
// remains independent of the owned migration connection. It does not exclude old business
// writers: the reviewed coordinator/host deployment locks and writer freeze do.
func Apply(ctx context.Context, p Plan, open func() (*sql.DB, error)) error {
	if err := Validate(p); err != nil {
		return err
	}
	lockDB, err := open()
	if err != nil {
		return ErrRefused
	}
	defer func() { _ = lockDB.Close() }()
	lock, err := lockDB.Conn(ctx)
	if err != nil {
		return ErrRefused
	}
	defer func() { _ = lock.Close() }()
	var acquired int
	lockName := "iam-m3-" + digest([]byte(p.Before.Database))[:48]
	if lock.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lockName).Scan(&acquired) != nil || acquired != 1 {
		return ErrRefused
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released sql.NullInt64
		_ = lock.QueryRowContext(cleanup, "SELECT RELEASE_LOCK(?)", lockName).Scan(&released)
	}()
	current, err := Read(ctx, lockDB)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(current)
	b, _ := json.Marshal(p.Before)
	if string(a) != string(b) {
		return ErrRefused
	}
	if p.Action == "schema" {
		migrateDB, err := open()
		if err != nil {
			return ErrRefused
		}
		defer func() { _ = migrateDB.Close() }()
		version, err := migration.RunMessagingOffline(ctx, migrateDB, p.Before.Version)
		if err != nil || version != 45 {
			return ErrRefused
		}
		after, err := Read(ctx, lockDB)
		if err != nil || after.Version != 45 {
			return ErrRefused
		}
		_, err = Build("select", p.StoreID, p.WriterEpoch, after)
		// Legacy pending rows may remain while only additive schema is installed.
		// Validate the inert installation without authorizing protocol selection.
		if err != nil && after.LegacyPending {
			after.LegacyPending = false
			_, err = Build("select", p.StoreID, p.WriterEpoch, after)
		}
		return err
	}
	tx, err := lockDB.BeginTx(ctx, nil)
	if err != nil {
		return ErrRefused
	}
	defer func() { _ = tx.Rollback() }()
	var protocol, store, epoch, route string
	var generation int64
	var version int
	if tx.QueryRowContext(ctx, "SELECT protocol_type,store_id,writer_epoch,route_revision,generation,schema_version FROM iam_authz_message_protocol WHERE owner='iam.mysql' FOR UPDATE").Scan(&protocol, &store, &epoch, &route, &generation, &version) != nil || protocol != "outbox" || store != "" || epoch != "" || route != "" || generation != 1 || version != 1 {
		return ErrRefused
	}
	var busy bool
	if tx.QueryRowContext(ctx, "SELECT (EXISTS(SELECT 1 FROM rm_outbox WHERE BINARY state <> 'published' LIMIT 1) OR EXISTS(SELECT 1 FROM domain_event_outbox WHERE BINARY status <> 'published' LIMIT 1))").Scan(&busy) != nil || busy {
		return ErrRefused
	}
	for _, name := range tables {
		if name == "iam_authz_message_protocol" {
			continue
		}
		if tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM `"+name+"` LIMIT 1)").Scan(&busy) != nil || busy {
			return ErrRefused
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO rm_tx_binding(owner,store_id,writer_epoch,schema_version) VALUES('iam.mysql',?,?,1)", p.StoreID, p.WriterEpoch); err != nil {
		return ErrRefused
	}
	result, err := tx.ExecContext(ctx, "UPDATE iam_authz_message_protocol SET protocol_type='transaction',store_id=?,writer_epoch=?,route_revision='m3-1',generation=2 WHERE owner='iam.mysql' AND protocol_type='outbox' AND generation=1 AND schema_version=1", p.StoreID, p.WriterEpoch)
	if err != nil {
		return ErrRefused
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return ErrRefused
	}
	if tx.Commit() != nil {
		return errors.New("IAM M3 protocol commit unknown; inspect exact binding before any further step")
	}
	after, err := Read(ctx, lockDB)
	if err != nil {
		return ErrRefused
	}
	if after.Binding == nil || *after.Binding != (Binding{"iam.mysql", p.StoreID, p.WriterEpoch, 1}) || after.Selection == nil || *after.Selection != (Selection{"transaction", p.StoreID, p.WriterEpoch, "m3-1", 2, 1}) {
		return ErrRefused
	}
	return nil
}

func Encode(p Plan) ([]byte, error) {
	if err := Validate(p); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(p)
	return append(raw, '\n'), err
}
func Decode(raw []byte, expected string) (Plan, error) {
	var p Plan
	if len(raw) > 65536 || digest(raw) != expected || json.Unmarshal(raw, &p) != nil {
		return p, ErrRefused
	}
	canonical, err := Encode(p)
	if err != nil || string(canonical) != string(raw) {
		return p, ErrRefused
	}
	return p, nil
}
