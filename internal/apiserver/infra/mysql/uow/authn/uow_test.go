package authn_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	appuow "github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	mysqluow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/authn"
)

func TestUnitOfWork_WithNilDBRunsCallbackWithZeroRepositories(t *testing.T) {
	uow := mysqluow.NewUnitOfWork(nil)

	called := false
	err := uow.WithinTx(context.Background(), func(tx appuow.TxRepositories) error {
		called = true
		require.Nil(t, tx.Accounts)
		require.Nil(t, tx.Credentials)
		require.Nil(t, tx.Users)
		return nil
	})

	require.NoError(t, err)
	require.True(t, called)
}
