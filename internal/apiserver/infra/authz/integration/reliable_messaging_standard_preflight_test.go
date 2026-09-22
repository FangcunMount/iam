//go:build reliable_messaging

package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/message"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	"github.com/stretchr/testify/require"
)

func TestReliableMessagingStandardPreflight(t *testing.T) {
	require.NotEmpty(t, os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"))
	db := authzdb.Open(t, true)
	ddl, err := os.ReadFile(os.Getenv("RM_IAM_OUTBOX_UPGRADE"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(ddl)).Error)
	require.NoError(t, db.Exec("CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO schema_migrations VALUES(38,FALSE)").Error)
	catalog, err := eventcatalog.Load(os.Getenv("RM_IAM_EVENTS_CATALOG"))
	require.NoError(t, err)
	stager, err := eventoutbox.NewStandardStager(eventcatalog.NewCatalog(catalog))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	inspect := func(limit int) eventoutbox.ReliablePreflightReport {
		t.Helper()
		r, err := eventoutbox.InspectReliableOutbox(ctx, db, "iam.authz.version.v2", limit)
		require.NoError(t, err)
		require.False(t, r.WriterExclusionVerified || r.CutoverAuthorized)
		encoded, err := json.Marshal(r)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "private-fixture")
		now, err := time.Parse(time.RFC3339Nano, r.DatabaseTime)
		require.NoError(t, err)
		_, offset := now.Zone()
		require.Equal(t, 8*3600, offset)
		require.WithinDuration(t, time.Now(), now, 5*time.Second)
		return r
	}
	initial := inspect(10)
	require.True(t, initial.SDKDataReady && initial.LegacyRollbackDataReady && initial.UnusedSchemaCanBeRemoved)
	canceled, stop := context.WithCancel(ctx)
	stop()
	gateErr := eventoutbox.CheckStandardDrained(canceled, db)
	require.ErrorIs(t, gateErr, context.Canceled)
	require.ErrorIs(t, gateErr, eventoutbox.ErrUnsafeMessagingHandoff, "an unreadable ownership check must not allow degraded startup")
	// These SQL fixtures describe data gates; they are not a drain rehearsal.
	require.NoError(t, db.Exec(`INSERT INTO domain_event_outbox(event_id,event_type,aggregate_type,aggregate_id,topic_name,payload_json,status,next_attempt_at,created_at,updated_at) VALUES('private-fixture','unknown','unknown','unknown','unknown','{}','pending',CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3))`).Error)
	legacy := inspect(10)
	require.False(t, legacy.SDKDataReady)
	require.True(t, legacy.LegacyRollbackDataReady)
	require.EqualValues(t, 1, legacy.LegacyUnfinished)
	require.ErrorIs(t, eventoutbox.CheckLegacyDrained(ctx, db), eventoutbox.ErrUnsafeMessagingHandoff)
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET status='published' WHERE event_id='private-fixture'").Error)
	uow := authzuow.NewUnitOfWork(db, nil, stager)
	evt := policy.NewVersionChangedEvent(2)
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error { return repos.Events.Stage(txctx, evt) }))
	pending := inspect(10)
	require.True(t, pending.SDKDataReady && pending.RecoveryRequired)
	require.False(t, pending.LegacyRollbackDataReady || pending.UnusedSchemaCanBeRemoved)
	require.EqualValues(t, 1, pending.Pending)
	require.ErrorIs(t, eventoutbox.CheckStandardDrained(ctx, db), eventoutbox.ErrUnsafeMessagingHandoff)
	pool, err := db.DB()
	require.NoError(t, err)
	store, err := sdkmysql.New(pool)
	require.NoError(t, err)
	claims, err := store.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	active := inspect(10)
	require.True(t, active.SDKDataReady && active.RecoveryRequired)
	require.EqualValues(t, 1, active.Publishing)
	require.Zero(t, active.InconsistentMetadata)
	require.NoError(t, db.Exec("UPDATE rm_outbox SET fingerprint=UNHEX(REPEAT('ab',32)) WHERE id=?", claims[0].RecordID).Error)
	corrupt := inspect(10)
	require.EqualValues(t, 1, corrupt.InvalidUnfinishedIntents)
	require.False(t, corrupt.SDKDataReady || corrupt.LegacyRollbackDataReady)
	require.NoError(t, db.Exec("UPDATE rm_outbox SET state='unknown' WHERE id=?", claims[0].RecordID).Error)
	unknown := inspect(10)
	require.EqualValues(t, 1, unknown.UnknownStatus)
	snapshot, err := eventoutbox.NewStandardStatusReader(db).OutboxStatusSnapshot(ctx, time.Now())
	require.NoError(t, err)
	require.Len(t, snapshot.Buckets, 1)
	require.Equal(t, "standard_unknown", snapshot.Buckets[0].Status)
	require.NoError(t, db.Exec("UPDATE rm_outbox SET state='published',claim_token=NULL,lease_until=NULL WHERE id=?", claims[0].RecordID).Error)
	published := inspect(10)
	require.True(t, published.SDKDataReady && published.LegacyRollbackDataReady)
	require.EqualValues(t, 1, published.PublishedContentIssues, "published anomalies reported, never replayed")
	require.False(t, published.UnusedSchemaCanBeRemoved, "published receipts must be retained")
	bounded := inspect(1)
	require.True(t, bounded.Truncated)
	require.False(t, bounded.SDKDataReady || bounded.LegacyRollbackDataReady || bounded.UnusedSchemaCanBeRemoved)
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET status='unknown' WHERE event_id='private-fixture'").Error)
	legacyUnknown := inspect(10)
	require.EqualValues(t, 1, legacyUnknown.LegacyUnknown)
	require.False(t, legacyUnknown.SDKDataReady || legacyUnknown.LegacyRollbackDataReady)
	// The historical table uses a case-insensitive collation. Handoff gates must
	// not treat malformed casing or trailing bytes as the terminal state.
	for _, state := range []string{"Published", "published "} {
		require.NoError(t, db.Exec("UPDATE domain_event_outbox SET status=? WHERE event_id='private-fixture'", state).Error)
		require.ErrorIs(t, eventoutbox.CheckLegacyDrained(ctx, db), eventoutbox.ErrUnsafeMessagingHandoff)
		snapshot, err := eventoutbox.NewStandardStatusReader(db).OutboxStatusSnapshot(ctx, time.Now())
		require.NoError(t, err)
		require.Len(t, snapshot.Buckets, 1)
		require.Equal(t, "legacy_unknown", snapshot.Buckets[0].Status)
	}
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET status='published' WHERE event_id='private-fixture'").Error)
	// A correct generic SDK fingerprint cannot establish valid IAM semantics.
	input := claims[0].Message.Input()
	input.ID = "private-fixture-invalid-policy"
	input.Payload = []byte(`{"version":0}`)
	invalidPolicy, err := message.New(input)
	require.NoError(t, err)
	tx, err := pool.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	appender, err := sdkmysql.Bind(tx)
	require.NoError(t, err)
	require.NoError(t, appender.Append(ctx, invalidPolicy, time.Now()))
	require.NoError(t, tx.Commit())
	invalid := inspect(10)
	require.EqualValues(t, 1, invalid.InvalidUnfinishedIntents)
	require.False(t, invalid.SDKDataReady || invalid.LegacyRollbackDataReady)
}
