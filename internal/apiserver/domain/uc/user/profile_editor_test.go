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

func TestProfileEditor_RenameSuccess(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	userEntity, err := user.NewUser("old", phone)
	require.NoError(t, err)
	userEntity.ID = meta.FromUint64(1)
	repo.UsersByID[userEntity.ID.Uint64()] = userEntity

	validator := &testhelpers.UserValidatorStub{}
	editor := user.NewProfileEditor(repo, validator)

	updated, err := editor.Rename(context.Background(), userEntity.ID, "new-name")

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, 1, validator.RenameCalls)
	assert.Equal(t, 1, repo.FindIDCalls)
}

func TestProfileEditor_RenameValidationError(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	userEntity, _ := user.NewUser("old", phone)
	userEntity.ID = meta.FromUint64(2)
	repo.UsersByID[userEntity.ID.Uint64()] = userEntity

	validator := &testhelpers.UserValidatorStub{RenameErr: errors.New("bad name")}
	editor := user.NewProfileEditor(repo, validator)

	updated, err := editor.Rename(context.Background(), userEntity.ID, "")

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Equal(t, 1, validator.RenameCalls)
	assert.Equal(t, 0, repo.FindIDCalls, "repository should not be touched when validation fails")
}

func TestProfileEditor_UpdateContact(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	userEntity, _ := user.NewUser("user", phone)
	userEntity.ID = meta.FromUint64(3)
	repo.UsersByID[userEntity.ID.Uint64()] = userEntity

	validator := &testhelpers.UserValidatorStub{}
	editor := user.NewProfileEditor(repo, validator)

	newPhone, err := meta.NewPhone("+8613112345678")
	require.NoError(t, err)
	newEmail, err := meta.NewEmail("user@example.com")
	require.NoError(t, err)

	updated, err := editor.UpdateContact(context.Background(), userEntity.ID, newPhone, newEmail)

	require.NoError(t, err)
	assert.Equal(t, newPhone, updated.Phone)
	assert.Equal(t, newEmail, updated.Email)
	assert.Equal(t, 1, validator.UpdateContactCalls)
	assert.Equal(t, 1, repo.FindIDCalls)
}

func TestProfileEditor_UpdateContactValidationError(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	userEntity, _ := user.NewUser("user", phone)
	userEntity.ID = meta.FromUint64(4)
	repo.UsersByID[userEntity.ID.Uint64()] = userEntity

	validator := &testhelpers.UserValidatorStub{UpdateContactErr: errors.New("duplicate")}
	editor := user.NewProfileEditor(repo, validator)

	newPhone, err := meta.NewPhone("+8613112345678")
	require.NoError(t, err)
	newEmail, err := meta.NewEmail("user@example.com")
	require.NoError(t, err)

	updated, err := editor.UpdateContact(context.Background(), userEntity.ID, newPhone, newEmail)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.Equal(t, 1, validator.UpdateContactCalls)
	// repository should still be consulted to load the user
	assert.Equal(t, 1, repo.FindIDCalls)
}

func TestProfileEditor_UpdateIDCard(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	userEntity, _ := user.NewUser("user", phone)
	userEntity.ID = meta.FromUint64(5)
	repo.UsersByID[userEntity.ID.Uint64()] = userEntity

	editor := user.NewProfileEditor(repo, &testhelpers.UserValidatorStub{})
	idCard, err := meta.NewIDCard("tester", "110101199003070011")
	require.NoError(t, err)

	updated, err := editor.UpdateIDCard(context.Background(), userEntity.ID, idCard)

	require.NoError(t, err)
	assert.True(t, updated.IDCard.Equal(idCard))
	assert.Equal(t, 1, repo.FindIDCalls)
}
