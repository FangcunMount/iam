//go:build reliable_messaging

package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	role "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/timezone"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/outbox"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReliableMessagingStandardUoW(t *testing.T) {
	require.NotEmpty(t, os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"))
	fixture := authzdb.Open(t, true)
	require.NoError(t, fixture.Exec(sdkmysql.Schema).Error)
	// The standard Appender must borrow the actual UTC+8 business transaction.
	cfg, err := driver.ParseDSN(fixture.Dialector.(*gormmysql.Dialector).DSN)
	require.NoError(t, err)
	cfg.Loc = timezone.Location
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["time_zone"] = "'+08:00'"
	connector, err := driver.NewConnector(cfg)
	require.NoError(t, err)
	pool := sql.OpenDB(connector)
	t.Cleanup(func() { _ = pool.Close() })
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: pool}), &gorm.Config{})
	require.NoError(t, err)
	catalog, err := eventcatalog.Load(os.Getenv("RM_IAM_EVENTS_CATALOG"))
	require.NoError(t, err)
	stager, err := eventoutbox.NewStandardStager(eventcatalog.NewCatalog(catalog))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	original := policy.NewVersionChangedEvent(2)
	require.ErrorIs(t, stager.Stage(ctx, original), dbmysql.ErrActiveTransactionRequired)
	uow := authzuow.NewUnitOfWork(db, nil, stager)
	businessRole, err := role.NewRole("rm-standard-commit", "commit")
	require.NoError(t, err)
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if err := repos.Roles.Create(txctx, &businessRole); err != nil {
			return err
		}
		if _, err := repos.PolicyVersions.Increment(txctx, "rm-standard", "commit"); err != nil {
			return err
		}
		return repos.Events.Stage(txctx, original)
	}))
	// Duplicate append delegates to the SDK's persisted identity protection.
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		return repos.Events.Stage(txctx, original)
	}))
	var count, version int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM rm_outbox").Scan(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM domain_event_outbox").Scan(&count).Error)
	require.Zero(t, count, "one business transaction must not write two Outboxes")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM authz_roles WHERE name='rm-standard-commit'").Scan(&count).Error)
	require.EqualValues(t, 1, count)
	conflictingRole, err := role.NewRole("rm-standard-conflict", "conflict")
	require.NoError(t, err)
	err = uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if err := repos.Roles.Create(txctx, &conflictingRole); err != nil {
			return err
		}
		if _, err := repos.PolicyVersions.Increment(txctx, "rm-standard", "conflict"); err != nil {
			return err
		}
		changed := event.Event[map[string]any]{BaseEvent: original.BaseEvent, Data: map[string]any{"version": 2, "extension": "conflict"}}
		return repos.Events.Stage(txctx, changed)
	})
	require.ErrorIs(t, err, outbox.ErrConflict)
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM authz_roles WHERE name='rm-standard-conflict'").Scan(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Raw("SELECT MAX(policy_version) FROM authz_policy_versions").Scan(&version).Error)
	require.EqualValues(t, 2, version)
	abort := errors.New("host rollback")
	err = uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if err := repos.Events.Stage(txctx, policy.NewVersionChangedEvent(3)); err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM rm_outbox").Scan(&count).Error)
	require.EqualValues(t, 1, count)
	store, err := sdkmysql.New(pool)
	require.NoError(t, err)
	claims, err := store.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1, "UTC+8 business write must be immediately due")
	require.Equal(t, original.EventID(), claims[0].Message.Input().ID)
	require.Equal(t, []byte(`{"version":2}`), claims[0].Message.Input().Payload)
	require.Equal(t, original.OccurredAt().In(timezone.Location).Format(time.RFC3339Nano), claims[0].Message.Input().OccurredAt)
	require.WithinDuration(t, time.Now().Add(time.Minute), claims[0].LeaseUntil, 5*time.Second)
	require.NoError(t, store.Confirm(ctx, claims[0]))
}
