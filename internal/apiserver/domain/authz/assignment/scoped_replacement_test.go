package assignment_test

import (
	a "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScopedReplacementChangesOnlyRequestedCompany(t *testing.T) {
	name := mustRoleName(t, "qs:assessment_operator")
	one, err := scope.New(1, scope.Stores, []meta.ID{10})
	require.NoError(t, err)
	two, err := scope.New(2, scope.Stores, []meta.ID{20})
	require.NoError(t, err)
	changed, err := scope.New(1, scope.Stores, []meta.ID{30})
	require.NoError(t, err)
	makeAssignment := func(id uint64, value scope.Scope) *a.Assignment {
		item, err := a.NewAssignment(a.SubjectTypeUser, 1, 10, a.WithGrantedBy("admin"), a.WithID(a.NewAssignmentID(id)), a.WithScope(value))
		require.NoError(t, err)
		return &item
	}
	current := []*a.Assignment{makeAssignment(1, one), makeAssignment(2, two)}
	bindings := []a.ManagedRoleBinding{{ID: 10, Name: name}}
	request := a.ScopedReplacementRequest{OrgID: 1, ManagedRoleNames: []string{name.String()}, Targets: []a.ScopedRoleGrant{{RoleName: name, Scope: one}}}
	plan, err := (a.ReplacementPolicy{}).PlanScoped(request, bindings, current)
	require.NoError(t, err)
	require.False(t, plan.Changed)
	request.Targets[0].Scope = changed
	plan, err = (a.ReplacementPolicy{}).PlanScoped(request, bindings, current)
	require.NoError(t, err)
	require.Equal(t, []a.AssignmentID{a.NewAssignmentID(1)}, plan.Revokes)
	require.Len(t, plan.Grants, 1)
	require.True(t, plan.Grants[0].Scope.ContainsStore(1, 30))
	require.False(t, plan.Grants[0].Scope.ContainsStore(2, 20))
	request.Targets = nil
	plan, err = (a.ReplacementPolicy{}).PlanScoped(request, bindings, current)
	require.NoError(t, err)
	require.Equal(t, []a.AssignmentID{a.NewAssignmentID(1)}, plan.Revokes)
	require.Empty(t, plan.Grants)
	request.Targets = []a.ScopedRoleGrant{{RoleName: name, Scope: two}}
	_, err = (a.ReplacementPolicy{}).PlanScoped(request, bindings, current)
	require.Error(t, err)
	request.Targets = nil
	current = append(current, mustAssignment(t, 3, 10))
	_, err = (a.ReplacementPolicy{}).PlanScoped(request, bindings, current)
	require.Error(t, err, "legacy scope cannot be inferred")
}
