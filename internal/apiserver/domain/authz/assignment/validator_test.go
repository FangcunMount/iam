package assignment_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	assignment "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Use shared testhelpers stubs to avoid duplication. Tests run as external package to avoid import cycles.

func TestValidateGrantAndRevokeCommands_Invalids(t *testing.T) {
	v := assignment.NewValidator(&testhelpers.AssignmentRepoStub{}, &testhelpers.RoleRepoStub{}, testhelpers.NewUserResolverStub())

	// empty grant command
	err := v.ValidateGrantParameters("", 0, 0, "", "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	// empty revoke command
	err = v.ValidateRevokeParameters("", 0, 0, "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	err = v.ValidateGrantParameters(assignment.SubjectTypeGroup, meta.FromUint64(100), meta.FromUint64(1), "t1", "1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	err = v.ValidateRevokeParameters(assignment.SubjectTypeService, meta.FromUint64(1), meta.FromUint64(1), "t1")
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
	v := assignment.NewValidator(&testhelpers.AssignmentRepoStub{}, &testhelpers.RoleRepoStub{}, testhelpers.NewUserResolverStub())
	// zero binding id
	err := v.ValidateRevokeByIDParameters(assignment.NewAssignmentID(0), "")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestCheckRoleExists_NotFoundAndTenantMismatch(t *testing.T) {
	// role not found -> should map to ErrRoleNotFound
	repoNotFound := &testhelpers.RoleRepoStub{R: nil, Err: perrors.WithCode(code.ErrRoleNotFound, "notfound")}
	v1 := assignment.NewValidator(&testhelpers.AssignmentRepoStub{}, repoNotFound, testhelpers.NewUserResolverStub())
	err := v1.CheckRoleExists(context.Background(), meta.FromUint64(100), "t1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrRoleNotFound))

	// tenant mismatch
	repo := &testhelpers.RoleRepoStub{R: &role.Role{TenantID: "other"}, Err: nil}
	v2 := assignment.NewValidator(&testhelpers.AssignmentRepoStub{}, repo, testhelpers.NewUserResolverStub())
	err = v2.CheckRoleExists(context.Background(), meta.FromUint64(100), "tenant-a")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}

func TestFindAssignmentBySubjectAndRole_FoundAndNotFound(t *testing.T) {
	a1 := &assignment.Assignment{SubjectType: assignment.SubjectTypeUser, SubjectID: meta.FromUint64(1), RoleID: meta.FromUint64(11), TenantID: "t"}
	repo := &testhelpers.AssignmentRepoStub{Assignments: []*assignment.Assignment{a1}, Err: nil}
	v := assignment.NewValidator(repo, &testhelpers.RoleRepoStub{}, testhelpers.NewUserResolverStub())

	asg, err := v.FindAssignmentBySubjectAndRole(context.Background(), assignment.SubjectTypeUser, meta.FromUint64(1), meta.FromUint64(11), "t")
	require.NoError(t, err)
	require.NotNil(t, asg)
	assert.Equal(t, meta.FromUint64(11), asg.RoleID)

	// not found
	repoEmpty := &testhelpers.AssignmentRepoStub{Assignments: []*assignment.Assignment{}, Err: nil}
	v2 := assignment.NewValidator(repoEmpty, &testhelpers.RoleRepoStub{}, testhelpers.NewUserResolverStub())
	asg2, err2 := v2.FindAssignmentBySubjectAndRole(context.Background(), assignment.SubjectTypeUser, meta.FromUint64(1), meta.FromUint64(99), "t")
	require.Error(t, err2)
	assert.Nil(t, asg2)
	assert.True(t, perrors.IsCode(err2, code.ErrAssignmentNotFound))
}

func TestCheckSubjectExists_OnlySupportsExistingUsers(t *testing.T) {
	userResolver := testhelpers.NewUserResolverStub(meta.FromUint64(123))

	v := assignment.NewValidator(&testhelpers.AssignmentRepoStub{}, &testhelpers.RoleRepoStub{}, userResolver)

	err := v.CheckSubjectExists(context.Background(), assignment.SubjectTypeGroup, meta.FromUint64(1), "t1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	err = v.CheckSubjectExists(context.Background(), assignment.SubjectTypeUser, meta.FromUint64(999), "t1")
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrUserNotFound))

	err = v.CheckSubjectExists(context.Background(), assignment.SubjectTypeUser, meta.FromUint64(123), "t1")
	require.NoError(t, err)
}
