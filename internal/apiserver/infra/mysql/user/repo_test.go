package user

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testhelpers "github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryFindByIDMapsRecordNotFoundToDomainCode(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&UserPO{}))

	repo := NewRepository(db)

	_, err := repo.FindByID(context.Background(), meta.FromUint64(404))
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrUserNotFound))
}

func TestUserRepositoryFindByPhoneReturnsNilWhenMissing(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&UserPO{}))

	repo := NewRepository(db)

	phone, err := meta.NewPhone("+8613900000000")
	require.NoError(t, err)

	got, err := repo.FindByPhone(context.Background(), phone)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestUserRepositoryFindByIDsReturnsFoundUsersOnly(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&UserPO{}))

	phone10, err := meta.NewPhone("+8613800000010")
	require.NoError(t, err)
	email10, err := meta.NewEmail("alice@example.com")
	require.NoError(t, err)
	phone11, err := meta.NewPhone("+8613800000011")
	require.NoError(t, err)
	email11, err := meta.NewEmail("bob@example.com")
	require.NoError(t, err)

	alice := &UserPO{Name: "alice", Phone: phone10, Email: email10, Status: 1}
	alice.ID = meta.FromUint64(10)
	bob := &UserPO{Name: "bob", Phone: phone11, Email: email11, Status: 1}
	bob.ID = meta.FromUint64(11)
	require.NoError(t, db.Create(alice).Error)
	require.NoError(t, db.Create(bob).Error)

	repo := NewRepository(db)
	got, err := repo.FindByIDs(context.Background(), []meta.ID{meta.FromUint64(11), meta.FromUint64(404), meta.FromUint64(10), meta.FromUint64(11)})

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "alice", got[meta.FromUint64(10)].Name)
	require.Equal(t, "bob", got[meta.FromUint64(11)].Name)
	require.Nil(t, got[meta.FromUint64(404)])
}
