package user_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	user "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestUniquenessChecker_CheckPhoneUniqueSuccess(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	checker := user.NewUniquenessChecker(repo)

	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	err = checker.CheckPhoneUnique(context.Background(), phone)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestUniquenessChecker_CheckPhoneUnique_EmptyPhone(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	checker := user.NewUniquenessChecker(repo)

	err := checker.CheckPhoneUnique(context.Background(), meta.Phone{})
	require.NoError(t, err)
	assert.Equal(t, 0, repo.FindPhoneCalls)
}

func TestUniquenessChecker_CheckPhoneUnique_DuplicatePhone(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	repo.UsersByPhone[phone.String()] = &user.User{}
	checker := user.NewUniquenessChecker(repo)

	err = checker.CheckPhoneUnique(context.Background(), phone)

	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "already exists")
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestUniquenessChecker_CheckPhoneUnique_ErrorPropagation(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	repo.PhoneErr = errors.New("db failure")
	checker := user.NewUniquenessChecker(repo)

	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	err = checker.CheckPhoneUnique(context.Background(), phone)

	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "check user phone")
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestUniquenessChecker_CheckPhoneChange(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	userEntity, _ := user.NewUser("user", phone)
	checker := user.NewUniquenessChecker(repo)

	// same phone should skip uniqueness check
	err = checker.CheckPhoneChange(context.Background(), userEntity, phone)
	require.NoError(t, err)
	assert.Equal(t, 0, repo.FindPhoneCalls)

	// changed phone and repository says available
	newPhone1, err := meta.NewPhone("+8613112345678")
	require.NoError(t, err)
	err = checker.CheckPhoneChange(context.Background(), userEntity, newPhone1)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)

	// changed phone but duplicate exists
	newPhone2, err := meta.NewPhone("+8613212345678")
	require.NoError(t, err)
	repo.UsersByPhone[newPhone2.String()] = &user.User{}
	err = checker.CheckPhoneChange(context.Background(), userEntity, newPhone2)
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "already exists")
	assert.Equal(t, 2, repo.FindPhoneCalls)
}

func TestUniquenessChecker_CheckPhoneUnique_NotFound(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	checker := user.NewUniquenessChecker(repo)

	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)

	err = checker.CheckPhoneUnique(context.Background(), phone)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestUniquenessChecker_CheckPhoneUnique_Found(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	repo.UsersByPhone[phone.String()] = &user.User{}
	checker := user.NewUniquenessChecker(repo)

	err = checker.CheckPhoneUnique(context.Background(), phone)

	require.Error(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
	assert.Contains(t, fmt.Sprintf("%-v", err), "already exists")
}

func TestUniquenessChecker_CheckPhoneUnique_RepoReturnsNotFound(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	phone, err := meta.NewPhone("+8613012345678")
	require.NoError(t, err)
	// ensure stub returns ErrRecordNotFound
	delete(repo.UsersByPhone, phone.String())
	checker := user.NewUniquenessChecker(repo)

	err = checker.CheckPhoneUnique(context.Background(), phone)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.FindPhoneCalls)
}

func TestUniquenessChecker_CheckPhoneUnique_RepoReturnsUnknown(t *testing.T) {
	repo := testhelpers.NewUserRepoStub()
	repo.PhoneErr = gorm.ErrInvalidDB
	checker := user.NewUniquenessChecker(repo)

	phone, err := meta.NewPhone("+8613412345678")
	require.NoError(t, err)

	err = checker.CheckPhoneUnique(context.Background(), phone)

	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%-v", err), "failed")
	assert.Equal(t, 1, repo.FindPhoneCalls)
}
