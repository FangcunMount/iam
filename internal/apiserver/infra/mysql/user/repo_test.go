package user

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testhelpers "github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryMapsRecordNotFoundToDomainCode(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&UserPO{}))

	repo := NewRepository(db)

	_, err := repo.FindByID(context.Background(), meta.FromUint64(404))
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrUserNotFound))

	phone, err := meta.NewPhone("+8613900000000")
	require.NoError(t, err)

	_, err = repo.FindByPhone(context.Background(), phone)
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrUserNotFound))
}
