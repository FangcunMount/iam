package migration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FangcunMount/reliable-messaging/message"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	"github.com/stretchr/testify/require"
)

// This branch is a fallback built from the deployed IAM application revision.
// It must boot with the retained schema 39 and continue to own SDK failure state.
func TestSchema39FallbackMigrationAndRetryMySQL(t *testing.T) {
	if os.Getenv("IAM_RM_FALLBACK_REQUIRED") == "1" {
		require.NotEmpty(t, os.Getenv("MYSQL_HOST"))
	}
	db := openMigrationMySQL(t)
	var tables int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()").Scan(&tables))
	require.Zero(t, tables, "disposable empty database required")

	for _, name := range []string{
		"000038_standard_message_outbox.up.sql",
		"000039_reliable_messaging_failure_state.up.sql",
	} {
		ddl, err := migrations.ReadFile("migrations/" + name)
		require.NoError(t, err)
		_, err = db.Exec(string(ddl))
		require.NoError(t, err)
	}
	_, err := db.Exec("CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO schema_migrations VALUES(39,FALSE)")
	require.NoError(t, err)

	// golang-migrate closes the pool supplied to its driver. Keep the Store's
	// connection independent so the fallback handoff is exercised afterwards.
	migrationDB := openMigrationMySQL(t)
	version, changed, err := NewMigrator(migrationDB, &Config{Enabled: true, Database: migrationEnvOr("MYSQL_DATABASE", "iam_test")}).Run()
	require.NoError(t, err, "the fallback binary must recognize the retained journal version")
	require.EqualValues(t, 39, version)
	require.False(t, changed)

	msg, err := message.New(message.Input{
		Producer: "iam", ID: "fallback-retry", Destination: "iam.authz.version.v2",
		EventType: "iam.authz.version_changed.v2", SchemaVersion: "v2", Scope: "global",
		ContentType: "application/json", OccurredAt: "2026-09-27T09:00:00+08:00", Payload: []byte("{}"),
	})
	require.NoError(t, err)
	fingerprint := msg.Fingerprint()
	_, err = db.Exec(`INSERT INTO rm_outbox(producer,message_id,destination,event_type,schema_version,scope,content_type,occurred_at,payload,fingerprint,state,next_attempt_at,failure_count)
		VALUES(?,?,?,?,?,?,?,?,?,?,'retry_wait',UTC_TIMESTAMP(6),2)`,
		"iam", "fallback-retry", "iam.authz.version.v2", "iam.authz.version_changed.v2", "v2", "global",
		"application/json", "2026-09-27T09:00:00+08:00", []byte("{}"), fingerprint[:])
	require.NoError(t, err)
	store, err := sdkmysql.New(db)
	require.NoError(t, err)
	claims, err := store.ClaimDue(context.Background(), 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.EqualValues(t, 2, claims[0].FailureCount)
	require.NoError(t, store.Retry(context.Background(), claims[0], time.Second, "fallback_probe"))
	var failures int
	var state string
	require.NoError(t, db.QueryRow("SELECT state,failure_count FROM rm_outbox WHERE message_id='fallback-retry'").Scan(&state, &failures))
	require.Equal(t, "retry_wait", state)
	require.Equal(t, 3, failures, "fallback must retain the v0.2.1 failure budget")
}
