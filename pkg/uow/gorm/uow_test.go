package gormuow_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	gormuow "github.com/FangcunMount/iam/v3/pkg/uow/gorm"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type uowTestRow struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string
}

func setupUoWDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&uowTestRow{}))
	return db
}

func TestUnitOfWorkCommitRollbackAndTxContext(t *testing.T) {
	db := setupUoWDB(t)
	uow := gormuow.NewUnitOfWork(db)

	err := uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		tx, err := gormuow.RequireTx(txCtx)
		require.NoError(t, err)
		return tx.Create(&uowTestRow{ID: 1, Name: "committed"}).Error
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&uowTestRow{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	rollbackErr := errors.New("rollback")
	err = uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		tx, err := gormuow.RequireTx(txCtx)
		require.NoError(t, err)
		require.NoError(t, tx.Create(&uowTestRow{ID: 2, Name: "rolled-back"}).Error)
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	require.NoError(t, db.Model(&uowTestRow{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestUnitOfWorkNestedRequiredReusesOuterTransaction(t *testing.T) {
	db := setupUoWDB(t)
	uow := gormuow.NewUnitOfWork(db)

	var hooks []string
	err := uow.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		outerTx, err := gormuow.RequireTx(txCtx)
		require.NoError(t, err)
		require.NoError(t, gormuow.AfterCommit(txCtx, func(context.Context) error {
			hooks = append(hooks, "outer")
			return nil
		}))

		return uow.WithinTransaction(txCtx, func(nestedCtx context.Context) error {
			nestedTx, err := gormuow.RequireTx(nestedCtx)
			require.NoError(t, err)
			require.Same(t, outerTx, nestedTx)
			require.NoError(t, gormuow.AfterCommit(nestedCtx, func(context.Context) error {
				hooks = append(hooks, "nested")
				return nil
			}))
			return nil
		})
	})
	require.NoError(t, err)
	require.Equal(t, []string{"outer", "nested"}, hooks)
}

func TestUnitOfWorkNilDBFailsClosed(t *testing.T) {
	uow := gormuow.NewUnitOfWork(nil)
	called := false

	err := uow.WithinTransaction(context.Background(), func(context.Context) error {
		called = true
		return nil
	})

	require.ErrorIs(t, err, gormuow.ErrUnitOfWorkUnavailable)
	require.False(t, called)
}

func TestAfterCommitRequiresTransaction(t *testing.T) {
	err := gormuow.AfterCommit(context.Background(), func(context.Context) error { return nil })
	require.ErrorIs(t, err, gormuow.ErrActiveTransactionRequired)
}
