package migration

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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
		b, e := migrations.ReadFile("migrations/" + name)
		require.NoError(t, e)
		return string(b)
	}
	_, err := db.Exec(read("000006_add_domain_event_outbox.up.sql"))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO domain_event_outbox(event_id,event_type,aggregate_type,aggregate_id,topic_name,payload_json,status,attempt_count,next_attempt_at) VALUES('historical','iam.authz.version_changed.v2','PolicyVersion','2','iam.authz.version.v2','{ "version":2 }','failed',7,UTC_TIMESTAMP(3))`)
	require.NoError(t, err)
	up := read("000038_reliable_messaging_claims.up.sql")
	down := read("000038_reliable_messaging_claims.down.sql")
	_, err = db.Exec(up)
	require.NoError(t, err)
	var payload, state string
	var failures, claimCount, claimVersion int
	var tokenNil, leaseNil, hashNil bool
	require.NoError(t, db.QueryRow(`SELECT payload_json,status,attempt_count,rm_claim_count,rm_claim_version,rm_claim_token IS NULL,rm_lease_until IS NULL,rm_fingerprint IS NULL FROM domain_event_outbox WHERE event_id='historical'`).Scan(&payload, &state, &failures, &claimCount, &claimVersion, &tokenNil, &leaseNil, &hashNil))
	require.Equal(t, `{ "version":2 }`, payload)
	require.Equal(t, "failed", state)
	require.Equal(t, 7, failures)
	require.Zero(t, claimCount)
	require.Zero(t, claimVersion)
	require.True(t, tokenNil && leaseNil && hashNil)
	// Unused schema rollback must preserve the historical row.
	_, err = db.Exec(down)
	require.NoError(t, err)
	_, err = db.Exec(up)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE domain_event_outbox SET rm_fingerprint=UNHEX(REPEAT('ab',32)),rm_claim_count=1,rm_claim_version=1,status='quarantined' WHERE event_id='historical'`)
	require.NoError(t, err)
	_, err = db.Exec(down)
	require.Error(t, err, "used schema cannot silently discard receipt/fencing data")
	var retained int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM domain_event_outbox WHERE event_id='historical' AND attempt_count=7 AND status='quarantined' AND rm_fingerprint=UNHEX(REPEAT('ab',32))`).Scan(&retained))
	require.Equal(t, 1, retained)
	var indexes int
	require.NoError(t, db.QueryRow(`SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='domain_event_outbox' AND INDEX_NAME IN ('idx_status_next_attempt_at','idx_outbox_status_rm_lease','idx_outbox_status_updated')`).Scan(&indexes))
	require.Equal(t, 3, indexes)
	var collation string
	require.NoError(t, db.QueryRow(`SELECT COLLATION_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='domain_event_outbox' AND COLUMN_NAME='rm_claim_token'`).Scan(&collation))
	require.Equal(t, "ascii_bin", collation)
}
