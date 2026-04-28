package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	user "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func TestLifecycler_StatusTransitions(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	userEntity, _ := user.NewUser("user", phone)
	userEntity.ID = meta.FromUint64(10)
	repo.UsersByID[userEntity.ID.Uint64()] = userEntity

	lifecycle := user.NewLifecycler(repo)

	activated, err := lifecycle.Activate(context.Background(), userEntity.ID)
	require.NoError(t, err)
	assert.True(t, activated.IsUsable())

	deactivated, err := lifecycle.Deactivate(context.Background(), userEntity.ID)
	require.NoError(t, err)
	assert.True(t, deactivated.IsInactive())

	blocked, err := lifecycle.Block(context.Background(), userEntity.ID)
	require.NoError(t, err)
	assert.True(t, blocked.IsBlocked())
}

func TestLifecycler_RepoError(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	repo.FindErr = errors.New("db error")
	lifecycle := user.NewLifecycler(repo)

	activated, err := lifecycle.Activate(context.Background(), meta.FromUint64(1))
	require.Error(t, err)
	assert.Nil(t, activated)
}
