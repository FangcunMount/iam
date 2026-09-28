package migration

import (
	"context"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReliableMessagingSchemaUpgradeAndRollbackMySQL(t *testing.T) {
	if os.Getenv("IAM_RM_MIGRATION_REQUIRED") == "1" {
		require.NotEmpty(t, os.Getenv("MYSQL_HOST"))
	}
	db := openMigrationMySQL(t)
	var tables int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()").Scan(&tables))
	require.Zero(t, tables, "disposable empty database required")
	read := func(name string) string {
		b, err := migrations.ReadFile("migrations/" + name)
		require.NoError(t, err)
		return string(b)
	}
	_, err := db.Exec(read("000006_add_domain_event_outbox.up.sql"))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO domain_event_outbox(event_id,event_type,aggregate_type,aggregate_id,topic_name,payload_json,status,attempt_count,next_attempt_at) VALUES('historical','iam.authz.version_changed.v2','PolicyVersion','2','iam.authz.version.v2','{ "version":2 }','failed',7,UTC_TIMESTAMP(3))`)
	require.NoError(t, err)
	up := read("000038_standard_message_outbox.up.sql")
	down := read("000038_standard_message_outbox.down.sql")
	_, err = db.Exec(up)
	require.NoError(t, err)
	assertLegacy := func() {
		t.Helper()
		var payload, state string
		var attempts, compatibilityColumns int
		require.NoError(t, db.QueryRow("SELECT payload_json,status,attempt_count FROM domain_event_outbox WHERE event_id='historical'").Scan(&payload, &state, &attempts))
		require.Equal(t, `{ "version":2 }`, payload)
		require.Equal(t, "failed", state)
		require.Equal(t, 7, attempts)
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='domain_event_outbox' AND COLUMN_NAME LIKE 'rm\\_%'").Scan(&compatibilityColumns))
		require.Zero(t, compatibilityColumns, "new migration must not add legacy claim fields")
	}
	assertLegacy()
	_, err = db.Exec(down)
	require.NoError(t, err)
	assertLegacy()
	_, err = db.Exec(up)
	require.NoError(t, err)
	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: db}), &gorm.Config{})
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO schema_migrations VALUES(37,FALSE)")
	require.NoError(t, err)
	_, err = eventoutbox.NewMaintenanceStager(context.Background(), gormDB, nil, "legacy")
	require.ErrorContains(t, err, "journaled standard Outbox", "unjournaled table cannot enable legacy maintenance")
	_, err = db.Exec("UPDATE schema_migrations SET version=38")
	require.NoError(t, err)
	legacy, err := eventoutbox.NewMaintenanceStager(context.Background(), gormDB, nil, "legacy")
	require.NoError(t, err, "legacy maintenance remains available on the additive schema during upgrade")
	require.IsType(t, &eventoutbox.Store{}, legacy)
	_, err = eventoutbox.NewMaintenanceStager(context.Background(), gormDB, nil, "standard")
	require.ErrorContains(t, err, "clean migration 39", "new standard writer must not run on schema 38")
	// Even confirmed standard rows are retained evidence, not disposable data.
	_, err = db.Exec(`INSERT INTO rm_outbox(producer,message_id,destination,event_type,schema_version,scope,content_type,occurred_at,payload,fingerprint,state,next_attempt_at) VALUES('iam','retained','events','changed','v2','scope:global','application/json','2026-09-22T08:00:00+08:00','{}',UNHEX(REPEAT('ab',32)),'published',UTC_TIMESTAMP(6))`)
	require.NoError(t, err)
	_, err = db.Exec(down)
	require.Error(t, err, "used table must not be dropped")
	var retained, indexes int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM rm_outbox WHERE message_id='retained' AND state='published' AND fingerprint=UNHEX(REPEAT('ab',32))").Scan(&retained))
	require.Equal(t, 1, retained)
	assertLegacy()
	require.NoError(t, db.QueryRow("SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='rm_outbox' AND INDEX_NAME IN ('identity_key','due_idx','lease_idx')").Scan(&indexes))
	require.Equal(t, 3, indexes)
	// The released v0.2.1 Store refuses migration 38 even when no row is due.
	store, err := sdkmysql.New(db)
	require.NoError(t, err)
	_, err = store.ClaimDue(context.Background(), 1, time.Minute)
	require.Error(t, err)
	up39 := read("000039_reliable_messaging_failure_state.up.sql")
	down39 := read("000039_reliable_messaging_failure_state.down.sql")
	_, err = db.Exec(up39)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE schema_migrations SET version=39")
	require.NoError(t, err)
	require.NoError(t, eventoutbox.CheckReliableSchema(context.Background(), gormDB))
	claims, err := store.ClaimDue(context.Background(), 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, claims)
	var failures int
	var updated time.Time
	require.NoError(t, db.QueryRow("SELECT failure_count,updated_at FROM rm_outbox WHERE message_id='retained'").Scan(&failures, &updated))
	require.Zero(t, failures, "old claim attempts cannot be inferred as failures")
	require.False(t, updated.IsZero())
	_, err = db.Exec(down39)
	require.Error(t, err, "used table must retain additive failure state on rollback")
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM rm_outbox WHERE message_id='retained' AND state='published'").Scan(&retained))
	require.Equal(t, 1, retained)

	// The deployed binary embeds migrations only through 38. golang-migrate
	// validates the current journal version against that embedded catalog before
	// it can report no change, so an unmodified old image is not a rollback target.
	oldCatalog, err := iofs.New(fstest.MapFS{
		"migrations/000038_old.up.sql":   &fstest.MapFile{Data: []byte("SELECT 1")},
		"migrations/000038_old.down.sql": &fstest.MapFile{Data: []byte("SELECT 1")},
	}, "migrations")
	require.NoError(t, err)
	var databaseName string
	require.NoError(t, db.QueryRow("SELECT DATABASE()").Scan(&databaseName))
	oldDatabase, err := migratemysql.WithInstance(db, &migratemysql.Config{
		DatabaseName: databaseName, MigrationsTable: "schema_migrations",
	})
	require.NoError(t, err)
	oldMigrator, err := migrate.NewWithInstance("iofs", oldCatalog, "mysql", oldDatabase)
	require.NoError(t, err)
	require.ErrorContains(t, oldMigrator.Up(), "no migration found for version 39")
}
