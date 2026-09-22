//go:build reliable_messaging

package integration_test

import (
	"context"
	"database/sql"
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
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReliableMessagingPreflightAndTimezone(t *testing.T) {
	require.NotEmpty(t, os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"))
	db := authzdb.Open(t, true)
	require.NoError(t, db.Exec("DROP TABLE domain_event_outbox").Error)
	for _, variable := range []string{"RM_IAM_OUTBOX_SCHEMA", "RM_IAM_OUTBOX_UPGRADE"} {
		ddl, err := os.ReadFile(os.Getenv(variable))
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(ddl)).Error)
	}
	require.NoError(t, db.Exec("CREATE TABLE schema_migrations(version BIGINT NOT NULL PRIMARY KEY, dirty BOOLEAN NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO schema_migrations VALUES(38,FALSE)").Error)
	cfg, err := driver.ParseDSN(db.Dialector.(*gormmysql.Dialector).DSN)
	require.NoError(t, err)
	cfg.Loc = time.FixedZone("test-UTC+8", 8*3600)
	connector, err := driver.NewConnector(cfg)
	require.NoError(t, err)
	pool := sql.OpenDB(connector)
	t.Cleanup(func() { _ = pool.Close() })
	other, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: pool}), &gorm.Config{})
	require.NoError(t, err)
	catalogConfig, err := eventcatalog.Load(os.Getenv("RM_IAM_EVENTS_CATALOG"))
	require.NoError(t, err)
	stager, err := eventoutbox.NewReliableStager(eventcatalog.NewCatalog(catalogConfig), 8*time.Hour)
	require.NoError(t, err)
	evt := policy.NewVersionChangedEvent(2)
	uow := authzuow.NewUnitOfWork(other, nil, stager)
	ctx := context.Background()
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		return repos.Events.Stage(txctx, evt)
	}))
	// The same DATETIME is parsed with different offsets by the two real pools.
	var utcRow, offsetRow eventoutbox.OutboxPO
	require.NoError(t, db.Where("event_id=?", evt.EventID()).First(&utcRow).Error)
	require.NoError(t, other.Where("event_id=?", evt.EventID()).First(&offsetRow).Error)
	require.Equal(t, utcRow.CreatedAt.Format("2006-01-02 15:04:05.000"), offsetRow.CreatedAt.Format("2006-01-02 15:04:05.000"))
	require.NotEqual(t, utcRow.CreatedAt.Unix(), offsetRow.CreatedAt.Unix())
	// Repeat staging via UTC must preserve the same digest and original row.
	uowUTC := authzuow.NewUnitOfWork(db, nil, stager)
	require.NoError(t, uowUTC.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error { return repos.Events.Stage(txctx, evt) }))
	first, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	second, err := eventoutbox.InspectReliableOutbox(ctx, other, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.NotEmpty(t, first.DatabaseTime)
	for _, reportTime := range []string{first.DatabaseTime, second.DatabaseTime} {
		parsed, parseErr := time.Parse(time.RFC3339Nano, reportTime)
		require.NoError(t, parseErr)
		_, offset := parsed.Zone()
		require.Equal(t, 8*3600, offset, "operator report must carry the agreed UTC+8 offset")
		require.WithinDuration(t, time.Now(), parsed, 5*time.Second, "formatting must preserve the database instant")
	}
	second.DatabaseTime = first.DatabaseTime // Compare counts from two successive snapshots.
	require.Equal(t, first, second, "preflight must not report timezone-only digest conflicts")
	require.True(t, first.SDKDataReady && first.LegacyRollbackDataReady)
	require.False(t, first.UnusedSchemaCanBeRemoved || first.CutoverAuthorized || first.WriterExclusionVerified)
	require.Zero(t, first.RowsWithoutFingerprint)
	store, err := eventoutbox.NewReliableStore(db, utcRow.TopicName, time.Minute, 8*time.Hour)
	require.NoError(t, err)
	// New Stage uses the audited historical offset on the database clock: no test-side adjustment
	// should be needed to make an offset-connection write immediately claimable.
	claims, err := store.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1, "valid timezone-crossing row must not be quarantined")
	active, err := eventoutbox.InspectReliableOutbox(ctx, other, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, active.ActiveSDKLeases)
	require.True(t, active.SDKDataReady && active.RecoveryRequired)
	require.False(t, active.LegacyRollbackDataReady)
	var unchanged eventoutbox.OutboxPO
	require.NoError(t, db.Where("event_id=?", evt.EventID()).First(&unchanged).Error)
	require.Equal(t, "publishing", unchanged.Status, "inspection must not settle or release a claim")
	require.NoError(t, store.Confirm(ctx, claims[0]))
	// Sequential handoff: an actual legacy Stager in UTC+8 writes a due row;
	// SDK takes it, records a retry using the same original-column clock, and
	// the legacy claimant can take that retry after SDK has released ownership.
	legacyStore := eventoutbox.NewStore(other, eventcatalog.NewCatalog(catalogConfig))
	legacyEvent := policy.NewVersionChangedEvent(3)
	legacyUoW := authzuow.NewUnitOfWork(other, nil, legacyStore)
	require.NoError(t, legacyUoW.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		return repos.Events.Stage(txctx, legacyEvent)
	}))
	legacyClaims, err := store.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, legacyClaims, 1, "SDK must not defer a UTC+8 legacy row by eight hours")
	require.NoError(t, store.Retry(ctx, legacyClaims[0], 50*time.Millisecond, "unknown"))
	require.Eventually(t, func() bool {
		rows, claimErr := legacyStore.ClaimDueEvents(ctx, 1, time.Now())
		if claimErr != nil || len(rows) != 1 {
			return false
		}
		return rows[0].EventID == legacyEvent.EventID()
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, legacyStore.MarkEventPublished(ctx, legacyEvent.EventID(), time.Now()))
	require.NoError(t, db.Exec(`UPDATE domain_event_outbox SET payload_json='{"version":99}' WHERE event_id=?`, evt.EventID()).Error)
	published, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, published.PublishedContentIssues)
	claims, err = store.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, claims, "published anomalies must never trigger automatic replay")
	invalid := utcRow
	invalid.ID = 0
	invalid.EventID = "private-event-sentinel"
	invalid.Status = "unexpected_state"
	invalid.PayloadJSON = `{"private":"payload-sentinel"}`
	require.NoError(t, db.Create(&invalid).Error)
	bad, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, bad.UnknownStatus)
	require.EqualValues(t, 1, bad.InvalidUnfinishedIntents)
	require.False(t, bad.SDKDataReady || bad.LegacyRollbackDataReady || bad.UnusedSchemaCanBeRemoved)
	encoded, err := json.Marshal(bad)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "sentinel")
	require.NoError(t, db.Exec(`UPDATE domain_event_outbox SET status='pending',payload_json=?,rm_claim_token='broken' WHERE id=?`, utcRow.PayloadJSON, invalid.ID).Error)
	metadata, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, metadata.InconsistentMetadata)
	require.False(t, metadata.SDKDataReady)
	require.NoError(t, db.Exec(`UPDATE domain_event_outbox SET rm_claim_token=NULL,rm_fingerprint=UNHEX(REPEAT('ab',32)) WHERE id=?`, invalid.ID).Error)
	conflict, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, conflict.UnfinishedContentConflicts)
	require.False(t, conflict.SDKDataReady)
	require.NoError(t, db.Exec(`UPDATE domain_event_outbox SET status='publishing',rm_fingerprint=NULL,rm_claim_token=REPEAT('a',32),rm_claim_count=1,rm_claim_version=1,rm_lease_until=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE id=?`, invalid.ID).Error)
	expired, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, expired.ExpiredSDKLeases)
	require.True(t, expired.SDKDataReady && expired.RecoveryRequired)
	require.False(t, expired.LegacyRollbackDataReady)
	require.NoError(t, db.Exec(`UPDATE domain_event_outbox SET status='quarantined',rm_claim_token=NULL,rm_lease_until=NULL WHERE id=?`, invalid.ID).Error)
	quarantine, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, quarantine.Quarantined)
	require.False(t, quarantine.SDKDataReady || quarantine.LegacyRollbackDataReady)
	partial, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 1)
	require.NoError(t, err)
	require.True(t, partial.Truncated)
	require.False(t, partial.SDKDataReady || partial.LegacyRollbackDataReady || partial.UnusedSchemaCanBeRemoved)
	// Imported/corrupt zero IDs must be inspected too, rather than disappearing
	// behind the initial keyset cursor. Session changes affect this fixture only.
	require.NoError(t, db.Connection(func(conn *gorm.DB) error {
		var mode string
		if err := conn.Raw("SELECT @@SESSION.sql_mode").Scan(&mode).Error; err != nil {
			return err
		}
		if err := conn.Exec("SET SESSION sql_mode=?", mode+",NO_AUTO_VALUE_ON_ZERO").Error; err != nil {
			return err
		}
		defer func() { require.NoError(t, conn.Exec("SET SESSION sql_mode=?", mode).Error) }()
		return conn.Exec(`INSERT INTO domain_event_outbox(id,event_id,event_type,aggregate_type,aggregate_id,topic_name,payload_json,status,next_attempt_at,created_at,updated_at)
		 SELECT 0,'zero-record',event_type,aggregate_type,aggregate_id,topic_name,?, 'pending',next_attempt_at,created_at,updated_at FROM domain_event_outbox WHERE id=?`, utcRow.PayloadJSON, utcRow.ID).Error
	}))
	zeroID, err := eventoutbox.InspectReliableOutbox(ctx, db, utcRow.TopicName, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, zeroID.InconsistentMetadata)
}
