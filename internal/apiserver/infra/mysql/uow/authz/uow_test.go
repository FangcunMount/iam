package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	appuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	mysqluow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/authz"
)

func TestUnitOfWork_WithNilDBRunsCallbackWithZeroRepositories(t *testing.T) {
	uow := mysqluow.NewUnitOfWork(nil)

	called := false
	err := uow.WithinTx(context.Background(), func(tx appuow.TxRepositories) error {
		called = true
		require.Nil(t, tx.Assignments)
		require.Nil(t, tx.Roles)
		require.Nil(t, tx.Resources)
		require.Nil(t, tx.PolicyVersions)
		require.Nil(t, tx.Users)
		require.Nil(t, tx.RuleStore)
		return nil
	})

	require.NoError(t, err)
	require.True(t, called)
}
