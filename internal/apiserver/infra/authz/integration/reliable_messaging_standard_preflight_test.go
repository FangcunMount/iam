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
	legacyoutbox "github.com/FangcunMount/iam/v5/internal/testutil/legacyoutbox"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/message"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openStandardPreflightDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NotEmpty(t, os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"))
	db := authzdb.Open(t, true)
	ddl, err := os.ReadFile(os.Getenv("RM_IAM_OUTBOX_UPGRADE"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(ddl)).Error)
	failureDDL, err := os.ReadFile(os.Getenv("RM_IAM_OUTBOX_FAILURE_UPGRADE"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(failureDDL)).Error)
	require.NoError(t, db.Exec("CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO schema_migrations VALUES(39,FALSE)").Error)
	return db
}

func TestReliableMessagingStandardPreflight(t *testing.T) {
	db := openStandardPreflightDB(t)
	catalog, err := eventcatalog.Load(os.Getenv("RM_IAM_EVENTS_CATALOG"))
	require.NoError(t, err)
	stager, err := legacyoutbox.NewStandardStager(eventcatalog.NewCatalog(catalog))
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
	defer func() { _ = tx.Rollback() }()
	appender, err := sdkmysql.Bind(tx)
	require.NoError(t, err)
	require.NoError(t, appender.Append(ctx, invalidPolicy, time.Now()))
	require.NoError(t, tx.Commit())
	invalid := inspect(10)
	require.EqualValues(t, 1, invalid.InvalidUnfinishedIntents)
	require.False(t, invalid.SDKDataReady || invalid.LegacyRollbackDataReady)
}

func TestIAMStandardOutboxRetainedHistory(t *testing.T) {
	db := openStandardPreflightDB(t)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	catalog, err := eventcatalog.Load(os.Getenv("RM_IAM_EVENTS_CATALOG"))
	require.NoError(t, err)
	stager, err := legacyoutbox.NewStandardStager(eventcatalog.NewCatalog(catalog))
	require.NoError(t, err)
	uow := authzuow.NewUnitOfWork(db, nil, stager)
	evt := policy.NewVersionChangedEvent(2)
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		return repos.Events.Stage(txctx, evt)
	}))
	reader := eventoutbox.NewStandardStatusReader(db)
	now := time.Now()
	const topic = "iam.authz.version.v2"

	require.ErrorIs(t, eventoutbox.CheckTransactionHandoff(ctx, db), eventoutbox.ErrUnsafeMessagingHandoff)
	require.NoError(t, db.Exec("UPDATE rm_outbox SET state='published', transport_confirmed_at=UTC_TIMESTAMP(6)").Error)
	require.NoError(t, eventoutbox.CheckTransactionHandoff(ctx, db))
	original, err := eventoutbox.InspectReliableOutbox(ctx, db, topic, 10)
	require.NoError(t, err)
	require.Equal(t, "rm_outbox", original.StandardSource)
	require.EqualValues(t, 1, original.Published)

	var before []string
	require.NoError(t, db.Raw("SELECT CONCAT_WS(':', id, HEX(message_id), HEX(payload), HEX(fingerprint), state, transport_confirmed_at) FROM rm_outbox").Scan(&before).Error)
	require.Len(t, before, 1)
	require.NoError(t, db.Exec("RENAME TABLE rm_outbox TO iam_rm_outbox_history").Error)
	var after []string
	require.NoError(t, db.Raw("SELECT CONCAT_WS(':', id, HEX(message_id), HEX(payload), HEX(fingerprint), state, transport_confirmed_at) FROM iam_rm_outbox_history").Scan(&after).Error)
	require.Equal(t, before, after)
	require.NoError(t, eventoutbox.CheckTransactionHandoff(ctx, db))
	require.Error(t, eventoutbox.CheckReliableSchema(ctx, db), "history never grants SDK write ownership")
	snapshot, err := reader.OutboxStatusSnapshot(ctx, now)
	require.NoError(t, err)
	require.Empty(t, snapshot.Buckets)
	history, err := eventoutbox.InspectReliableOutbox(ctx, db, topic, 10)
	require.NoError(t, err)
	require.Equal(t, "iam_rm_outbox_history", history.StandardSource)
	require.Equal(t, original.Published, history.Published)
	require.False(t, history.SDKDataReady || history.LegacyRollbackDataReady || history.UnusedSchemaCanBeRemoved ||
		history.WriterExclusionVerified || history.CutoverAuthorized)

	for state, bucket := range map[string]string{"pending": "history_pending", "Published": "history_unknown", "unknown": "history_unknown"} {
		require.NoError(t, db.Exec("UPDATE iam_rm_outbox_history SET state=?", state).Error)
		require.ErrorIs(t, eventoutbox.CheckTransactionHandoff(ctx, db), eventoutbox.ErrUnsafeMessagingHandoff)
		snapshot, err = reader.OutboxStatusSnapshot(ctx, now)
		require.NoError(t, err)
		require.Len(t, snapshot.Buckets, 1)
		require.EqualValues(t, 1, snapshot.Buckets[0].Count)
		require.Equal(t, bucket, snapshot.Buckets[0].Status)
	}
	require.NoError(t, db.Exec("UPDATE iam_rm_outbox_history SET state='published'").Error)

	assertRefused := func() {
		t.Helper()
		require.Error(t, eventoutbox.CheckTransactionHandoff(ctx, db))
		_, err := reader.OutboxStatusSnapshot(ctx, now)
		require.Error(t, err)
		_, err = eventoutbox.InspectReliableOutbox(ctx, db, topic, 10)
		require.Error(t, err)
	}
	require.NoError(t, db.Exec("CREATE TABLE rm_outbox LIKE iam_rm_outbox_history").Error)
	assertRefused()
	require.NoError(t, db.Exec("DROP TABLE rm_outbox").Error)
	require.NoError(t, db.Exec("RENAME TABLE iam_rm_outbox_history TO saved_standard_history").Error)
	assertRefused()
	require.NoError(t, db.Exec("CREATE VIEW iam_rm_outbox_history AS SELECT * FROM saved_standard_history").Error)
	assertRefused()
	require.NoError(t, db.Exec("DROP VIEW iam_rm_outbox_history").Error)
	require.NoError(t, db.Exec("CREATE TABLE iam_rm_outbox_history (id BIGINT PRIMARY KEY, state VARCHAR(24))").Error)
	assertRefused()
}
