package migration

import (
	"context"
	"os"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messagefailure"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	"github.com/stretchr/testify/require"
)

func TestIAMNSQFailureAuditMigrationAndDedupMySQL(t *testing.T) {
	if os.Getenv("IAM_RM_MIGRATION_REQUIRED") == "1" {
		require.NotEmpty(t, os.Getenv("MYSQL_HOST"))
	}
	db := openMigrationMySQL(t)
	var existing int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()`).Scan(&existing))
	require.Zero(t, existing, "dedicated empty test database required")
	read := func(name string) string {
		body, err := migrations.ReadFile("migrations/" + name)
		require.NoError(t, err)
		return string(body)
	}
	up := read("000040_iam_nsq_failure_audit.up.sql")
	down := read("000040_iam_nsq_failure_audit.down.sql")
	_, err := db.Exec(up)
	require.NoError(t, err)
	_, err = db.Exec(down)
	require.NoError(t, err, "empty table may be removed")
	_, err = db.Exec(up)
	require.NoError(t, err)

	audit, err := messagefailure.New(db)
	require.NoError(t, err)
	failure := legacy.FailedHandoff{
		Topic: "iam.authz.version.v2", Channel: "iam-policy-sync.first.1#ephemeral",
		UUID: "application-id", TransportMessageID: "physical-1",
		Metadata: map[string]string{"event_type": "iam.authz.version_changed.v2"},
		Payload:  []byte(`{"version":7}`), Attempts: 5, Timestamp: 123456789, Cause: "reload failed",
	}
	ctx := context.Background()
	require.NoError(t, audit.Record(ctx, failure))
	failure.TransportMessageID = "physical-2"
	failure.Cause = "reloaded process failed"
	require.NoError(t, audit.Record(ctx, failure), "duplicate physical handoff retains one application failure")
	var count, seen int
	var firstID, lastID, firstCause, lastCause, topic, channel, applicationID string
	require.NoError(t, db.QueryRow(`
SELECT COUNT(*),MAX(seen_count),MAX(first_transport_id),MAX(last_transport_id),
 MAX(first_cause),MAX(last_cause),MAX(topic),MAX(channel_name),MAX(application_id)
FROM iam_nsq_failure_audit`).Scan(&count, &seen, &firstID, &lastID, &firstCause, &lastCause, &topic, &channel, &applicationID))
	require.Equal(t, 1, count)
	require.Equal(t, 2, seen)
	require.Equal(t, "physical-1", firstID)
	require.Equal(t, "physical-2", lastID)
	require.Equal(t, "reload failed", firstCause)
	require.Equal(t, "reloaded process failed", lastCause)
	require.Equal(t, failure.Topic, topic)
	require.Equal(t, failure.Channel, channel)
	require.Equal(t, failure.UUID, applicationID)

	changed := failure
	changed.Payload = []byte(`{"version":8}`)
	require.ErrorContains(t, audit.Record(ctx, changed), "identity reused with different content")
	require.NoError(t, db.QueryRow("SELECT seen_count FROM iam_nsq_failure_audit").Scan(&seen))
	require.Equal(t, 2, seen, "conflicting payload cannot be acknowledged as duplicate")
	_, err = db.Exec(down)
	require.Error(t, err, "rollback must preserve audited failures")
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM iam_nsq_failure_audit").Scan(&count))
	require.Equal(t, 1, count)

	_, err = db.Exec("DROP TABLE iam_nsq_failure_audit") // disposable test database only
	require.NoError(t, err)
	require.Error(t, audit.Record(ctx, failure), "missing table cannot authorize failure FIN")
}
