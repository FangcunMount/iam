package eventoutbox_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/internal/apiserver/outboxcore"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOutboxStore(t *testing.T) (*gorm.DB, *eventoutbox.Store, *eventcatalog.Catalog) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&eventoutbox.OutboxPO{}))
	catalog := testOutboxCatalog(t)
	return db, eventoutbox.NewStore(db, catalog), catalog
}

func testOutboxCatalog(t *testing.T) *eventcatalog.Catalog {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`
version: "1"
topics:
  authz_version:
    name: iam.authz.version
  notification_sms:
    name: iam.notify.sms
events:
  iam.authz.version_changed:
    topic: authz_version
    delivery: durable_outbox
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
  iam.login_otp_sms:
    topic: notification_sms
    delivery: best_effort
    aggregate: LoginOTP
    domain: authn
    handler: sms-dispatcher
`))
	require.NoError(t, err)
	return eventcatalog.NewCatalog(cfg)
}

func versionEvent(version int) event.DomainEvent {
	return event.New(eventcatalog.AuthzVersionChanged, "PolicyVersion", "tenant-a", map[string]any{
		"tenant_id": "tenant-a",
		"version":   version,
	})
}

func TestStageRequiresActiveTransaction(t *testing.T) {
	_, store, _ := setupOutboxStore(t)

	err := store.Stage(context.Background(), versionEvent(1))

	require.ErrorIs(t, err, mysql.ErrActiveTransactionRequired)
}

func TestStageCommitsAndRollsBackWithUnitOfWork(t *testing.T) {
	db, store, _ := setupOutboxStore(t)
	uow := mysql.NewUnitOfWork(db)

	err := uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		return store.Stage(txCtx, versionEvent(1))
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	err = uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		require.NoError(t, store.Stage(txCtx, versionEvent(2)))
		return context.Canceled
	})
	require.ErrorIs(t, err, context.Canceled)

	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestStageRejectsBestEffortEvents(t *testing.T) {
	db, store, _ := setupOutboxStore(t)
	uow := mysql.NewUnitOfWork(db)
	bestEffort := event.New(eventcatalog.LoginOTPSMS, "LoginOTP", "+8613800138000", map[string]string{"code": "123456"})

	err := uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		return store.Stage(txCtx, bestEffort)
	})

	require.ErrorContains(t, err, "cannot be staged to outbox")
}

func TestClaimAndMarkEventLifecycle(t *testing.T) {
	db, store, _ := setupOutboxStore(t)
	uow := mysql.NewUnitOfWork(db)
	require.NoError(t, uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		return store.Stage(txCtx, versionEvent(3))
	}))

	now := time.Now().Add(time.Second)
	claimed, err := store.ClaimDueEvents(context.Background(), 10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, eventcatalog.AuthzVersionChanged, claimed[0].EventType)
	require.Equal(t, "iam.authz.version", claimed[0].TopicName)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(claimed[0].Payload, &payload))
	require.Equal(t, "tenant-a", payload["tenant_id"])
	require.Equal(t, float64(3), payload["version"])

	var row eventoutbox.OutboxPO
	require.NoError(t, db.Where("event_id = ?", claimed[0].EventID).First(&row).Error)
	require.Equal(t, outboxcore.StatusPublishing, row.Status)

	publishedAt := now.Add(time.Second)
	require.NoError(t, store.MarkEventPublished(context.Background(), claimed[0].EventID, publishedAt))
	require.NoError(t, db.Where("event_id = ?", claimed[0].EventID).First(&row).Error)
	require.Equal(t, outboxcore.StatusPublished, row.Status)
	require.NotNil(t, row.PublishedAt)
}

func TestClaimReclaimsStalePublishingEvents(t *testing.T) {
	db, store, _ := setupOutboxStore(t)
	uow := mysql.NewUnitOfWork(db)
	require.NoError(t, uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		return store.Stage(txCtx, versionEvent(4))
	}))

	now := time.Now()
	claimed, err := store.ClaimDueEvents(context.Background(), 10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	reclaimed, err := store.ClaimDueEvents(context.Background(), 10, now.Add(outboxcore.DefaultPublishingStaleFor+time.Second))
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.Equal(t, claimed[0].EventID, reclaimed[0].EventID)
}

func TestMarkUnknownEventReturnsError(t *testing.T) {
	_, store, _ := setupOutboxStore(t)

	err := store.MarkEventPublished(context.Background(), "missing", time.Now())
	require.ErrorContains(t, err, "not found")

	err = store.MarkEventFailed(context.Background(), "missing", "failed", time.Now())
	require.ErrorContains(t, err, "not found")
}

func TestOutboxStatusSnapshotCountsUnfinishedStatuses(t *testing.T) {
	db, store, _ := setupOutboxStore(t)
	now := time.Now()
	require.NoError(t, db.Create([]eventoutbox.OutboxPO{
		statusRow("evt-pending", outboxcore.StatusPending, now.Add(-3*time.Minute)),
		statusRow("evt-failed", outboxcore.StatusFailed, now.Add(-2*time.Minute)),
		statusRow("evt-publishing", outboxcore.StatusPublishing, now.Add(-time.Minute)),
		statusRow("evt-published", outboxcore.StatusPublished, now.Add(-time.Minute)),
	}).Error)

	snapshot, err := store.OutboxStatusSnapshot(context.Background(), now)
	require.NoError(t, err)

	require.Equal(t, "iam-mysql-outbox", snapshot.Store)
	require.Len(t, snapshot.Buckets, 3)
	countByStatus := map[string]int64{}
	for _, bucket := range snapshot.Buckets {
		countByStatus[bucket.Status] = bucket.Count
		require.GreaterOrEqual(t, bucket.OldestAgeSeconds, 0.0)
	}
	require.Equal(t, int64(1), countByStatus[outboxcore.StatusPending])
	require.Equal(t, int64(1), countByStatus[outboxcore.StatusFailed])
	require.Equal(t, int64(1), countByStatus[outboxcore.StatusPublishing])
}

func statusRow(eventID, status string, createdAt time.Time) eventoutbox.OutboxPO {
	return eventoutbox.OutboxPO{
		EventID:       eventID,
		EventType:     eventcatalog.AuthzVersionChanged,
		AggregateType: "PolicyVersion",
		AggregateID:   "tenant-a",
		TopicName:     "iam.authz.version",
		PayloadJSON:   `{"tenant_id":"tenant-a","version":1}`,
		Status:        status,
		NextAttemptAt: createdAt,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
}
