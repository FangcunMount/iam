//go:build integration

package migration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messageprotocol"
	hostuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	database "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	eventoutbox "github.com/FangcunMount/iam/v5/internal/testutil/legacyoutbox"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/transaction"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Only the caller's fresh owned schema is selected. Runtime constructors never
// select a protocol; production selection belongs to the separate offline tool.
func selectOwnedTransactionProtocol(t *testing.T, db *sql.DB, binding transaction.Binding) {
	t.Helper()
	result, err := db.ExecContext(t.Context(), `UPDATE iam_authz_message_protocol SET protocol_type='transaction',store_id=?,writer_epoch=?,route_revision='m3-1',generation=2 WHERE owner='iam.mysql' AND protocol_type='outbox' AND generation=1`, binding.StoreID, binding.WriterEpoch)
	require.NoError(t, err)
	changed, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
}

func testM3ProtocolSelection(t *testing.T, host *gorm.DB, binding transaction.Binding) {
	ctx := t.Context()
	require.NoError(t, messageprotocol.RequireTransaction(ctx, host, binding, "m3-1", false))
	require.ErrorIs(t, messageprotocol.RequireOutbox(ctx, host, false), messageprotocol.ErrSelection)
	for _, wrong := range []transaction.Binding{
		{Owner: "qs.mysql", StoreID: binding.StoreID, WriterEpoch: binding.WriterEpoch},
		{Owner: binding.Owner, StoreID: "wrong", WriterEpoch: binding.WriterEpoch},
		{Owner: binding.Owner, StoreID: binding.StoreID, WriterEpoch: "old"},
	} {
		require.ErrorIs(t, messageprotocol.RequireTransaction(ctx, host, wrong, "m3-1", false), messageprotocol.ErrSelection)
	}
	require.ErrorIs(t, messageprotocol.RequireTransaction(ctx, host, binding, "old", false), messageprotocol.ErrSelection)

	t.Run("old-stager-rolls-back-original-facts", func(t *testing.T) {
		cfg, err := eventcatalog.Load("../../../configs/events.yaml")
		require.NoError(t, err)
		stager, err := eventoutbox.NewStandardStager(eventcatalog.NewCatalog(cfg))
		require.NoError(t, err)
		before := int64(0)
		require.NoError(t, host.Table("rm_outbox").Count(&before).Error)
		root := hostuow.NewUnitOfWork(host, nil, stager)
		err = root.WithinTx(ctx, func(active context.Context, repos appuow.TxRepositories) error {
			tx, err := database.RequireTx(active)
			if err != nil {
				return err
			}
			if err = tx.Exec("INSERT INTO iam_m3_local_facts VALUES ('rejected-old-writer')").Error; err != nil {
				return err
			}
			return repos.Events.Stage(active, policy.NewVersionChangedEvent(900001))
		})
		require.ErrorIs(t, err, messageprotocol.ErrSelection)
		var rows int64
		require.NoError(t, host.Table("iam_m3_local_facts").Where("operation_id=?", "rejected-old-writer").Count(&rows).Error)
		require.Zero(t, rows)
		require.NoError(t, host.Table("rm_outbox").Count(&rows).Error)
		require.Equal(t, before, rows)
	})
	t.Run("selection-update-cannot-cross-live-root", func(t *testing.T) {
		root := host.Begin()
		require.NoError(t, root.Error)
		defer root.Rollback()
		require.NoError(t, messageprotocol.RequireTransaction(ctx, root, binding, "m3-1", true))
		// A canceled autocommit UPDATE can still commit after the client loses
		// its reply. Use an owned root and a server-reported lock timeout, then
		// explicitly roll back, before asserting unchanged selection.
		writer := host.Begin()
		require.NoError(t, writer.Error)
		defer writer.Rollback()
		require.NoError(t, writer.Exec("SET SESSION innodb_lock_wait_timeout=1").Error)
		err := writer.Exec("UPDATE iam_authz_message_protocol SET protocol_type='outbox',generation=generation+1 WHERE owner='iam.mysql'").Error
		var timeout *driver.MySQLError
		require.ErrorAs(t, err, &timeout)
		require.EqualValues(t, 1205, timeout.Number, "selection crossed a root's shared publication gate")
		require.NoError(t, writer.Rollback().Error)
		require.NoError(t, root.Rollback().Error)
		require.NoError(t, messageprotocol.RequireTransaction(ctx, host, binding, "m3-1", false))
		require.NoError(t, host.Exec("UPDATE iam_authz_message_protocol SET protocol_type='outbox',generation=generation+1 WHERE owner='iam.mysql'").Error)
		defer func() {
			require.NoError(t, host.Exec("UPDATE iam_authz_message_protocol SET protocol_type='transaction',generation=generation+1 WHERE owner='iam.mysql'").Error)
		}()
		require.NoError(t, messageprotocol.RequireOutbox(ctx, host, false))
		local, err := hostuow.NewTransactionUnitOfWork(host, nil, binding)
		require.NoError(t, err)
		called := false
		err = local.ReadPlan(ctx, func(context.Context, appuow.TxRepositories) error { called = true; return nil })
		require.ErrorIs(t, err, messageprotocol.ErrSelection)
		require.False(t, called)
	})
	t.Run("missing-installed-row-fails-closed", func(t *testing.T) {
		root := host.Begin()
		require.NoError(t, root.Error)
		defer root.Rollback()
		require.NoError(t, root.Exec("DELETE FROM iam_authz_message_protocol WHERE owner='iam.mysql'").Error)
		require.ErrorIs(t, messageprotocol.RequireOutbox(ctx, root, true), messageprotocol.ErrSelection)
		require.ErrorIs(t, messageprotocol.RequireTransaction(ctx, root, binding, "m3-1", true), messageprotocol.ErrSelection)
		require.NoError(t, root.Rollback().Error)
	})
	t.Run("missing-installed-table-fails-closed", func(t *testing.T) {
		require.NoError(t, host.Exec("RENAME TABLE iam_authz_message_protocol TO iam_m3_retained_protocol").Error)
		defer func() {
			require.NoError(t, host.Exec("RENAME TABLE iam_m3_retained_protocol TO iam_authz_message_protocol").Error)
		}()
		require.ErrorIs(t, messageprotocol.RequireOutbox(ctx, host, false), messageprotocol.ErrSelection)
		require.True(t, errors.Is(messageprotocol.RequireTransaction(ctx, host, binding, "m3-1", false), messageprotocol.ErrSelection))
	})
}
