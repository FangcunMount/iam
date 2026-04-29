package user_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	user "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func TestValidator_ValidateCreateSuccess(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	v := user.NewValidator(repo)

	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	err = v.ValidateCreate(context.Background(), " name ", phone)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestValidator_ValidateCreate_EmptyInputs(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	v := user.NewValidator(repo)

	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	err = v.ValidateCreate(context.Background(), " ", phone)
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")

	err = v.ValidateCreate(context.Background(), "name", meta.Phone{})
	require.NoError(t, err)
	assert.Equal(t, 0, repo.FindPhoneCalls)
}

func TestValidator_ValidateCreate_DuplicatePhone(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	repo.UsersByPhone[phone.String()] = &user.User{}
	v := user.NewValidator(repo)

	err = v.ValidateCreate(context.Background(), "name", phone)

	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "already exists")
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestValidator_CheckPhoneUnique_ErrorPropagation(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	repo.PhoneErr = errors.New("db failure")
	v := user.NewValidator(repo)

	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	err = v.CheckPhoneUnique(context.Background(), phone)

	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "check user phone")
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestValidator_CheckPhoneUnique_EmptyPhone(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	v := user.NewValidator(repo)

	err := v.CheckPhoneUnique(context.Background(), meta.Phone{})
	require.NoError(t, err)
	assert.Equal(t, 0, repo.FindPhoneCalls)
}

func TestValidator_ValidateRename(t *testing.T) {
	v := user.NewValidator(testhelpers.NewUserRepoStub())

	assert.NoError(t, v.ValidateRename(" valid "))

	err := v.ValidateRename(" ")
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "name cannot be empty")
}

func TestValidator_ValidateUpdateContact(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	userEntity, _ := user.NewUser("user", phone)
	v := user.NewValidator(repo)

	email, err := meta.NewEmail("a@b.com")
	require.NoError(t, err)

	// same phone should skip uniqueness check
	err = v.ValidateUpdateContact(context.Background(), userEntity, phone, email)
	require.NoError(t, err)
	assert.Equal(t, 0, repo.FindPhoneCalls)

	// changed phone and repository says available
	newPhone1, err := meta.NewPhone("+8613112345678")
	require.NoError(t, err)
	err = v.ValidateUpdateContact(context.Background(), userEntity, newPhone1, email)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)

	// changed phone but duplicate exists
	newPhone2, err := meta.NewPhone("+8613212345678")
	require.NoError(t, err)
	repo.UsersByPhone[newPhone2.String()] = &user.User{}
	err = v.ValidateUpdateContact(context.Background(), userEntity, newPhone2, email)
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "already exists")
	assert.Equal(t, 2, repo.FindPhoneCalls)
}

func TestValidator_CheckPhoneUnique_NotFound(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	v := user.NewValidator(repo)

	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	err = v.CheckPhoneUnique(context.Background(), phone)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestValidator_CheckPhoneUnique_Found(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	repo.UsersByPhone[phone.String()] = &user.User{}
	v := user.NewValidator(repo)

	err = v.CheckPhoneUnique(context.Background(), phone)

	require.Error(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
	assert.Contains(t, fmt.Sprintf("%-v", err), "already exists")
}

func TestValidator_CheckPhoneUnique_RepoReturnsNotFound(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	// ensure stub returns ErrRecordNotFound
	delete(repo.UsersByPhone, phone.String())
	v := user.NewValidator(repo)

	err = v.CheckPhoneUnique(context.Background(), phone)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestValidator_CheckPhoneUnique_RepoReturnsUnknown(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	repo.PhoneErr = gorm.ErrInvalidDB
	v := user.NewValidator(repo)

	phone, err := meta.NewPhone("+8613412345678")
	require.NoError(t, err)

	err = v.CheckPhoneUnique(context.Background(), phone)

	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "failed")
	assert.Equal(t, 1, repo.FindPhoneCalls)
}
