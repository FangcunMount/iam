//go:build reliable_messaging

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
)

func TestReliableMessagingMaintenanceStager(t *testing.T) {
	require.NotEmpty(t, os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"))
	db := authzdb.Open(t, true)
	cfg, err := eventcatalog.Load(os.Getenv("RM_IAM_EVENTS_CATALOG"))
	require.NoError(t, err)
	catalog := eventcatalog.NewCatalog(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, db.Exec("CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO schema_migrations VALUES(37,FALSE)").Error)
	selectStager := func(mode string) (event.Stager, error) {
		return eventoutbox.NewMaintenanceStager(ctx, db, catalog, mode)
	}
	legacy, err := selectStager("")
	require.NoError(t, err)
	require.IsType(t, &eventoutbox.Store{}, legacy, "pre-M3 maintenance retains its original writer")
	_, err = selectStager("standard")
	require.Error(t, err, "standard mode cannot run before migration 38")
	_, err = selectStager("STANDARD")
	require.ErrorContains(t, err, "outbox-mode")
	require.NoError(t, db.Exec("UPDATE schema_migrations SET dirty=TRUE").Error)
	_, err = selectStager("legacy")
	require.ErrorContains(t, err, "clean migration journal")
	require.NoError(t, db.Exec("UPDATE schema_migrations SET dirty=FALSE").Error)
	ddl, err := os.ReadFile(os.Getenv("RM_IAM_OUTBOX_UPGRADE"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(ddl)).Error)
	_, err = selectStager("")
	require.ErrorContains(t, err, "explicit --outbox-mode", "unjournaled standard table must not silently choose legacy")
	require.NoError(t, db.Exec("UPDATE schema_migrations SET version=38").Error)
	_, err = selectStager("")
	require.ErrorContains(t, err, "explicit --outbox-mode", "schema alone cannot select the live Relay")
	standard, err := selectStager("standard")
	require.NoError(t, err)
	require.IsType(t, &eventoutbox.StandardStager{}, standard)
	legacy, err = selectStager("legacy")
	require.NoError(t, err)
	// Maintenance uses the original host UoW and insert-only Stager, without a
	// separate transaction or Relay. Actual scheduling/transport has other proofs.
	uow := dbmysql.NewUnitOfWork(db)
	oldEvent := policy.NewVersionChangedEvent(2)
	require.NoError(t, uow.WithinTransaction(ctx, func(txctx context.Context) error {
		return legacy.Stage(txctx, oldEvent)
	}))
	_, err = selectStager("standard")
	require.ErrorIs(t, err, eventoutbox.ErrUnsafeMessagingHandoff)
	for _, state := range []string{"failed", "publishing", "quarantined", "unknown", "Published", "published "} {
		require.NoError(t, db.Exec("UPDATE domain_event_outbox SET status=?", state).Error)
		_, err = selectStager("standard")
		require.ErrorIs(t, err, eventoutbox.ErrUnsafeMessagingHandoff)
	}
	// Synthetic terminal states test data gates, not a real drain or cutover.
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET status='published'").Error)
	standard, err = selectStager("standard")
	require.NoError(t, err)
	abort := errors.New("abort maintenance transaction")
	err = uow.WithinTransaction(ctx, func(txctx context.Context) error {
		if e := standard.Stage(txctx, policy.NewVersionChangedEvent(3)); e != nil {
			return e
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM rm_outbox").Scan(&count).Error)
	require.Zero(t, count, "rolled-back maintenance leaves no notification")
	require.NoError(t, uow.WithinTransaction(ctx, func(txctx context.Context) error {
		return standard.Stage(txctx, policy.NewVersionChangedEvent(3))
	}))
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM domain_event_outbox").Scan(&count).Error)
	require.EqualValues(t, 1, count, "standard maintenance does not append historical work")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM rm_outbox").Scan(&count).Error)
	require.EqualValues(t, 1, count)
	for _, state := range []string{"pending", "retry_wait", "publishing", "quarantined", "unknown", "Published", "published "} {
		require.NoError(t, db.Exec("UPDATE rm_outbox SET state=?", state).Error)
		_, err = selectStager("legacy")
		require.ErrorIs(t, err, eventoutbox.ErrUnsafeMessagingHandoff)
	}
	require.NoError(t, db.Exec("UPDATE rm_outbox SET state='published'").Error)
	_, err = selectStager("legacy")
	require.NoError(t, err)
	canceled, stop := context.WithCancel(ctx)
	stop()
	_, err = eventoutbox.NewMaintenanceStager(canceled, db, catalog, "legacy")
	require.ErrorIs(t, err, context.Canceled)
	// A dropped table on a migrated database cannot reactivate an implicit writer.
	require.NoError(t, db.Exec("RENAME TABLE rm_outbox TO retained_rm_outbox").Error)
	_, err = selectStager("")
	require.ErrorContains(t, err, "explicit --outbox-mode")
	_, err = selectStager("standard")
	require.Error(t, err)
	_, err = selectStager("legacy")
	require.Error(t, err, "missing standard evidence must not enable a fallback writer")
}
