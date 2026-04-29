package uc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	appuow "github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	mysqluow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/uc"
	dbmysql "github.com/FangcunMount/iam/internal/pkg/database/mysql"
)

func TestUnitOfWork_WithNilDBFailsClosed(t *testing.T) {
	uow := mysqluow.NewUnitOfWork(nil)

	called := false
	err := uow.WithinTx(context.Background(), func(txCtx context.Context, tx appuow.TxRepositories) error {
		called = true
		return nil
	})

	require.ErrorIs(t, err, dbmysql.ErrUnitOfWorkUnavailable)
	require.False(t, called)
}
