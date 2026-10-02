package eventoutbox_test

import (
	"context"
	"strings"
	"testing"

	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/pkg/eventcodec"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOutboxStore(t *testing.T) (*gorm.DB, *eventoutbox.BootstrapStager, *eventcatalog.Catalog) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&eventoutbox.OutboxPO{}))
	catalog := testOutboxCatalog(t)
	stager, err := eventoutbox.NewBootstrapStager(catalog)
	require.NoError(t, err)
	return db, stager, catalog
}

func testOutboxCatalog(t *testing.T) *eventcatalog.Catalog {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`
version: "1"
topics:
  authz_version:
    name: iam.authz.version.v2
  notification_sms:
    name: iam.notify.sms
events:
  iam.authz.version_changed.v2:
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
	return policy.NewVersionChangedEvent(int64(version))
}

func TestStageRequiresActiveTransaction(t *testing.T) {
	_, store, _ := setupOutboxStore(t)

	err := store.Stage(context.Background(), versionEvent(1))

	require.ErrorIs(t, err, mysql.ErrActiveTransactionRequired)
}

func TestStageCommitsAndRollsBackWithUnitOfWork(t *testing.T) {
	db, store, _ := setupOutboxStore(t)
	uow := mysql.NewUnitOfWork(db)

	evt := versionEvent(1)
	err := uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		return store.Stage(txCtx, evt)
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	var row eventoutbox.OutboxPO
	require.NoError(t, db.First(&row).Error)
	payload, err := eventcodec.EncodePayload(evt)
	require.NoError(t, err)
	require.Equal(t, evt.EventID(), row.EventID)
	require.Equal(t, string(payload), row.PayloadJSON)
	require.Equal(t, evt.AggregateID(), row.AggregateID)
	require.Equal(t, "pending", row.Status)

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
	bestEffort := event.New(eventing.LoginOTPSMS, "LoginOTP", "+8613800138000", map[string]string{"code": "123456"})

	err := uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		return store.Stage(txCtx, bestEffort)
	})

	require.ErrorContains(t, err, "unsupported policy event")
}
