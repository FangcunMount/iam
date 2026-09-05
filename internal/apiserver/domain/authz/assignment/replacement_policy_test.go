package assignment_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestReplacementPolicyPreservesUnmanagedAssignments(t *testing.T) {
	managed := []assignmentDomain.ManagedRoleBinding{
		{Name: mustRoleName(t, "example:evaluator"), ID: meta.FromUint64(10)},
		{Name: mustRoleName(t, "example:staff"), ID: meta.FromUint64(11)},
	}
	current := []*assignmentDomain.Assignment{
		mustAssignment(t, 1, 10),
		mustAssignment(t, 2, 99),
	}

	plan, err := assignmentDomain.ReplacementPolicy{}.Plan(
		assignmentDomain.ReplacementRequest{
			TargetRoleNames:  []string{"example:staff"},
			ManagedRoleNames: []string{"example:staff", "example:evaluator"},
		},
		managed,
		current,
	)
	require.NoError(t, err)
	require.True(t, plan.Changed)
	require.Equal(t, []roleDomain.Name{mustRoleName(t, "example:staff")}, plan.DirectRoles)
	require.Equal(t, []assignmentDomain.AssignmentID{assignmentDomain.NewAssignmentID(1)}, plan.Revokes)
	require.Equal(t, []roleDomain.Name{mustRoleName(t, "example:staff")}, plan.Grants)
}

func TestReplacementPolicyNoOpWhenTargetMatchesCurrentManaged(t *testing.T) {
	managed := []assignmentDomain.ManagedRoleBinding{
		{Name: mustRoleName(t, "example:evaluator"), ID: meta.FromUint64(10)},
	}
	current := []*assignmentDomain.Assignment{mustAssignment(t, 1, 10)}

	plan, err := assignmentDomain.ReplacementPolicy{}.Plan(
		assignmentDomain.ReplacementRequest{
			TargetRoleNames:  []string{"example:evaluator"},
			ManagedRoleNames: []string{"example:evaluator"},
		},
		managed,
		current,
	)
	require.NoError(t, err)
	require.False(t, plan.Changed)
	require.Empty(t, plan.Revokes)
	require.Empty(t, plan.Grants)
}

func TestReplacementPolicyEmptyTargetRevokesAllManaged(t *testing.T) {
	managed := []assignmentDomain.ManagedRoleBinding{
		{Name: mustRoleName(t, "example:evaluator"), ID: meta.FromUint64(10)},
	}
	current := []*assignmentDomain.Assignment{mustAssignment(t, 1, 10)}

	plan, err := assignmentDomain.ReplacementPolicy{}.Plan(
		assignmentDomain.ReplacementRequest{ManagedRoleNames: []string{"example:evaluator"}},
		managed,
		current,
	)
	require.NoError(t, err)
	require.True(t, plan.Changed)
	require.Len(t, plan.Revokes, 1)
	require.Empty(t, plan.Grants)
}

func TestReplacementPolicyRejectsDuplicateAndUnmanagedTargets(t *testing.T) {
	managed := []assignmentDomain.ManagedRoleBinding{
		{Name: mustRoleName(t, "example:staff"), ID: meta.FromUint64(10)},
	}
	_, err := assignmentDomain.ReplacementPolicy{}.Plan(
		assignmentDomain.ReplacementRequest{
			TargetRoleNames:  []string{"example:staff", "example:staff"},
			ManagedRoleNames: []string{"example:staff"},
		},
		managed,
		nil,
	)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = assignmentDomain.ReplacementPolicy{}.Plan(
		assignmentDomain.ReplacementRequest{
			TargetRoleNames:  []string{"tenant_admin"},
			ManagedRoleNames: []string{"example:staff"},
		},
		managed,
		nil,
	)
	require.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}

func TestReplacementPolicyRevokesAreDeterministic(t *testing.T) {
	managed := []assignmentDomain.ManagedRoleBinding{
		{Name: mustRoleName(t, "example:a"), ID: meta.FromUint64(10)},
		{Name: mustRoleName(t, "example:b"), ID: meta.FromUint64(11)},
		{Name: mustRoleName(t, "example:c"), ID: meta.FromUint64(12)},
	}
	current := []*assignmentDomain.Assignment{
		mustAssignment(t, 30, 10),
		mustAssignment(t, 10, 11),
		mustAssignment(t, 20, 12),
	}
	plan, err := assignmentDomain.ReplacementPolicy{}.Plan(
		assignmentDomain.ReplacementRequest{ManagedRoleNames: []string{"example:a", "example:b", "example:c"}},
		managed,
		current,
	)
	require.NoError(t, err)
	require.Equal(t, []assignmentDomain.AssignmentID{
		assignmentDomain.NewAssignmentID(10),
		assignmentDomain.NewAssignmentID(20),
		assignmentDomain.NewAssignmentID(30),
	}, plan.Revokes)
}

func mustRoleName(t *testing.T, value string) roleDomain.Name {
	t.Helper()
	name, err := roleDomain.NewName(value)
	require.NoError(t, err)
	return name
}

func mustAssignment(t *testing.T, assignmentID uint64, roleID uint64) *assignmentDomain.Assignment {
	t.Helper()
	assignment, err := assignmentDomain.NewAssignment(
		assignmentDomain.SubjectTypeUser,
		meta.FromUint64(100),
		meta.FromUint64(roleID),
		"fangcun",
		assignmentDomain.WithID(assignmentDomain.NewAssignmentID(assignmentID)),
		assignmentDomain.WithGrantedBy("operator"),
	)
	require.NoError(t, err)
	return &assignment
}
