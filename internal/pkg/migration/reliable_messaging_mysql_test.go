package migration

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
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
}
