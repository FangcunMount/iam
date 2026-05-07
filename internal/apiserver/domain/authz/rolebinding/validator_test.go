package rolebinding_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	binding "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Use shared testhelpers stubs to avoid duplication. Tests run as external package to avoid import cycles.

func TestValidateGrantAndRevokeCommands_Invalids(t *testing.T) {
	v := binding.NewValidator(&testhelpers.BindingRepoStub{}, &testhelpers.RoleRepoStub{}, testhelpers.NewUserRepoStub())

	// empty grant command
	err := v.ValidateGrantParameters("", 0, 0, "", "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	// empty revoke command
	err = v.ValidateRevokeParameters("", 0, 0, "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	err = v.ValidateGrantParameters(binding.SubjectTypeGroup, meta.FromUint64(100), meta.FromUint64(1), "t1", "1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	err = v.ValidateRevokeParameters(binding.SubjectTypeService, meta.FromUint64(1), meta.FromUint64(1), "t1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	// validate list queries
	err = v.ValidateListBySubjectQuery(0, "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	err = v.ValidateListByRoleQuery(0, "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestValidateRevokeByIDParameters_Invalid(t *testing.T) {
	v := binding.NewValidator(&testhelpers.BindingRepoStub{}, &testhelpers.RoleRepoStub{}, testhelpers.NewUserRepoStub())
	// zero binding id
	err := v.ValidateRevokeByIDParameters(binding.NewBindingID(0), "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestCheckRoleExists_NotFoundAndTenantMismatch(t *testing.T) {
	// role not found -> should map to ErrRoleNotFound
	repoNotFound := &testhelpers.RoleRepoStub{R: nil, Err: perrors.WithCode(code.ErrRoleNotFound, "notfound")}
	v1 := binding.NewValidator(&testhelpers.BindingRepoStub{}, repoNotFound, testhelpers.NewUserRepoStub())
	err := v1.CheckRoleExists(context.Background(), meta.FromUint64(100), "t1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrRoleNotFound))

	// tenant mismatch
	repo := &testhelpers.RoleRepoStub{R: &role.Role{TenantID: "other"}, Err: nil}
	v2 := binding.NewValidator(&testhelpers.BindingRepoStub{}, repo, testhelpers.NewUserRepoStub())
	err = v2.CheckRoleExists(context.Background(), meta.FromUint64(100), "tenant-a")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}

func TestFindBindingBySubjectAndRole_FoundAndNotFound(t *testing.T) {
	a1 := &binding.Binding{SubjectType: binding.SubjectTypeUser, SubjectID: meta.FromUint64(1), RoleID: meta.FromUint64(11), TenantID: "t"}
	repo := &testhelpers.BindingRepoStub{Bindings: []*binding.Binding{a1}, Err: nil}
	v := binding.NewValidator(repo, &testhelpers.RoleRepoStub{}, testhelpers.NewUserRepoStub())

	asg, err := v.FindBindingBySubjectAndRole(context.Background(), binding.SubjectTypeUser, meta.FromUint64(1), meta.FromUint64(11), "t")
	require.NoError(t, err)
	require.NotNil(t, asg)
	assert.Equal(t, meta.FromUint64(11), asg.RoleID)

	// not found
	repoEmpty := &testhelpers.BindingRepoStub{Bindings: []*binding.Binding{}, Err: nil}
	v2 := binding.NewValidator(repoEmpty, &testhelpers.RoleRepoStub{}, testhelpers.NewUserRepoStub())
	asg2, err2 := v2.FindBindingBySubjectAndRole(context.Background(), binding.SubjectTypeUser, meta.FromUint64(1), meta.FromUint64(99), "t")
	require.Error(t, err2)
	assert.Nil(t, asg2)
	assert.True(t, perrors.IsCode(err2, code.ErrAssignmentNotFound))
}

func TestCheckSubjectExists_OnlySupportsExistingUsers(t *testing.T) {
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[123] = &userDomain.User{ID: meta.FromUint64(123)}

	v := binding.NewValidator(&testhelpers.BindingRepoStub{}, &testhelpers.RoleRepoStub{}, userRepo)

	err := v.CheckSubjectExists(context.Background(), binding.SubjectTypeGroup, meta.FromUint64(1), "t1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	err = v.CheckSubjectExists(context.Background(), binding.SubjectTypeUser, meta.FromUint64(999), "t1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrUserNotFound))

	err = v.CheckSubjectExists(context.Background(), binding.SubjectTypeUser, meta.FromUint64(123), "t1")
	require.NoError(t, err)
}
