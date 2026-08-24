package useraccess_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestServicePublishesUserLifecycleWithoutLeakingRepository(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	repo.UsersByID[1] = mustUser(t, 1, user.UserActive)
	repo.UsersByID[2] = mustUser(t, 2, user.UserInactive)
	repo.UsersByID[3] = mustUser(t, 3, user.UserBlocked)
	service := useraccess.NewService(repo)

	for _, tc := range []struct {
		id   uint64
		want useraccess.Status
	}{
		{id: 1, want: useraccess.StatusActive},
		{id: 2, want: useraccess.StatusInactive},
		{id: 3, want: useraccess.StatusBlocked},
		{id: 404, want: useraccess.StatusMissing},
	} {
		got, err := service.ReadUserStatus(context.Background(), meta.FromUint64(tc.id))
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
}

func TestServiceResolvesStableUserAnchor(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	repo.UsersByID[1] = mustUser(t, 1, user.UserBlocked)
	service := useraccess.NewService(repo)

	require.NoError(t, service.ResolveUser(context.Background(), meta.FromUint64(1)), "existence is independent of lifecycle status")
	err := service.ResolveUser(context.Background(), meta.FromUint64(404))
	require.True(t, perrors.IsCode(err, code.ErrUserNotFound))
	err = service.ResolveUser(context.Background(), meta.FromUint64(0))
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func mustUser(t *testing.T, id uint64, status user.Status) *user.User {
	t.Helper()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	u, err := user.NewUser("user", phone, user.WithID(meta.FromUint64(id)), user.WithStatus(status))
	require.NoError(t, err)
	return u
}
