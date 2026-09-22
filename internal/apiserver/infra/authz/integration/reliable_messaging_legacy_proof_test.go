//go:build reliable_messaging

package integration_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	"github.com/FangcunMount/reliable-messaging/message"
	"github.com/FangcunMount/reliable-messaging/outbox"
	"github.com/stretchr/testify/require"
)

// SQL feasibility prototype on the original table, not a production migration
// or a complete SDK Store implementation. The legacy writer remains unchanged.
func TestReliableMessagingHistoricalFencing(t *testing.T) {
	if os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN") == "" {
		t.Fatal("isolated MySQL DSN required")
	}
	db := authzdb.Open(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stager := authzdb.Stager(t, db)
	uow := authzuow.NewUnitOfWork(db, nil, stager)
	event := policy.NewVersionChangedEvent(2)
	require.NoError(t, uow.WithinTx(ctx, func(txCtx context.Context, repos appuow.TxRepositories) error {
		return repos.Events.Stage(txCtx, event)
	}))
	var original eventoutbox.OutboxPO
	require.NoError(t, db.Where("event_id = ?", event.EventID()).First(&original).Error)
	require.NoError(t, db.Exec(`ALTER TABLE domain_event_outbox
 ADD COLUMN rm_claim_token VARCHAR(64) NULL,
 ADD COLUMN rm_claim_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN rm_claim_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN rm_lease_until DATETIME(6) NULL`).Error)
	// Old stager writes remain compatible with the additive columns and defaults.
	next := policy.NewVersionChangedEvent(3)
	require.NoError(t, uow.WithinTx(ctx, func(txCtx context.Context, repos appuow.TxRepositories) error { return repos.Events.Stage(txCtx, next) }))
	// The historical attempt_count counts failed publications, not lease claims.
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET status='failed',attempt_count=7 WHERE event_id=?", event.EventID()).Error)
	result := db.Exec(`UPDATE domain_event_outbox SET status='publishing',rm_claim_token=?,rm_claim_version=rm_claim_version+1,rm_claim_count=rm_claim_count+1,rm_lease_until=UTC_TIMESTAMP(6)+INTERVAL 1 MINUTE,updated_at=UTC_TIMESTAMP(6)
 WHERE event_id=? AND status IN ('pending','failed') AND next_attempt_at<=UTC_TIMESTAMP(6)`, "old-token", event.EventID())
	require.NoError(t, result.Error)
	require.EqualValues(t, 1, result.RowsAffected)
	var row eventoutbox.OutboxPO
	require.NoError(t, db.Where("event_id=?", event.EventID()).First(&row).Error)
	require.Equal(t, original.PayloadJSON, row.PayloadJSON)
	require.Equal(t, 7, row.AttemptCount)
	intent, err := message.New(message.Input{Producer: "iam", ID: row.EventID, Destination: row.TopicName, EventType: row.EventType, SchemaVersion: "v2", Scope: "scope:global", ContentType: "application/json", OccurredAt: row.CreatedAt.UTC().Format(time.RFC3339Nano), Payload: []byte(row.PayloadJSON)})
	require.NoError(t, err)
	old := outbox.Claim{RecordID: strconv.FormatUint(row.ID, 10), Token: "old-token", Version: 1, Message: intent}
	// Explicit forced-expiry SQL, not a process-crash/natural-time experiment.
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET rm_lease_until=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE event_id=?", event.EventID()).Error)
	result = db.Exec(`UPDATE domain_event_outbox SET rm_claim_token='new-token',rm_claim_version=rm_claim_version+1,rm_claim_count=rm_claim_count+1,rm_lease_until=UTC_TIMESTAMP(6)+INTERVAL 1 MINUTE
 WHERE event_id=? AND status='publishing' AND rm_lease_until<=UTC_TIMESTAMP(6)`, event.EventID())
	require.NoError(t, result.Error)
	require.EqualValues(t, 1, result.RowsAffected)
	confirm := func(claim outbox.Claim) int64 {
		updated := db.Exec(`UPDATE domain_event_outbox SET status='published',published_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6),rm_claim_token=NULL,rm_lease_until=NULL,rm_claim_version=rm_claim_version+1
 WHERE id=? AND status='publishing' AND rm_claim_token=? AND rm_claim_version=? AND rm_lease_until>UTC_TIMESTAMP(6)`, claim.RecordID, claim.Token, claim.Version)
		require.NoError(t, updated.Error)
		return updated.RowsAffected
	}
	require.Zero(t, confirm(old))
	fresh := old
	fresh.Token = "new-token"
	fresh.Version = 2
	require.EqualValues(t, 1, confirm(fresh))
	require.NoError(t, db.Where("event_id=?", event.EventID()).First(&row).Error)
	require.Equal(t, "published", row.Status)
	require.Equal(t, 7, row.AttemptCount)
	require.Equal(t, string(intent.Input().Payload), row.PayloadJSON)
	// Critical negative evidence: the old API ignores all new fencing fields.
	// Its late failure can overwrite even the new published state. Mixed writers
	// must be stopped/drained or upgraded before cutover; added columns alone fail.
	legacy := eventoutbox.NewStore(db, nil)
	require.NoError(t, legacy.MarkEventFailed(ctx, event.EventID(), "late legacy writer", time.Now().Add(time.Hour)))
	require.NoError(t, db.Where("event_id=?", event.EventID()).First(&row).Error)
	require.Equal(t, "failed", row.Status)
	require.Equal(t, 8, row.AttemptCount)
	t.Log("new conditional writes fence stale claims, but original ID-only writer bypasses them: exclusive handoff remains mandatory")
}
