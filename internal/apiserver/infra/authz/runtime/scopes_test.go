package runtime_test

import (
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	rt "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSnapshotUnionsOnlyAssignmentsGrantingTheAction(t *testing.T) {
	data := assessmentDataset(t)
	a, err := scope.New(1, scope.Stores, []meta.ID{10})
	require.NoError(t, err)
	b, err := scope.New(1, scope.Stores, []meta.ID{20})
	require.NoError(t, err)
	other, err := scope.New(2, scope.AllStores, nil)
	require.NoError(t, err)
	data.Assignments = []rt.AssignmentRecord{{SubjectKey: "user:2", RoleID: 12, Scope: &a}, {SubjectKey: "user:2", RoleID: 13, Scope: &b}, {SubjectKey: "user:2", RoleID: 13, Scope: &other}}
	snapshot, err := rt.BuildSnapshot(data, time.Now())
	require.NoError(t, err)
	// The published snapshot must not retain caller-owned scope pointers.
	a = other
	sub, err := subject.ParseRef("user:2")
	require.NoError(t, err)
	result, err := snapshot.SubjectSnapshot(sub, "qs")
	require.NoError(t, err)
	for _, entry := range result.Permissions {
		switch entry.Action {
		case "retry":
			require.Len(t, entry.Scopes, 2)
			require.True(t, entry.Scopes[0].ContainsStore(1, 10))
			require.True(t, entry.Scopes[0].ContainsStore(1, 20))
			require.True(t, entry.Scopes[1].ContainsStore(2, 99))
			require.False(t, entry.Scopes[0].ContainsStore(2, 99))
		case "batch_evaluate":
			require.Len(t, entry.Scopes, 1)
			require.True(t, entry.Scopes[0].ContainsStore(1, 10))
			require.False(t, entry.Scopes[0].ContainsStore(1, 20), "plan role must not lend its store range to batch action")
		default:
			t.Fatalf("unexpected action %s", entry.Action)
		}
	}
	require.Len(t, result.Permissions, 2)
}

func TestSnapshotRejectsExplicitZeroScope(t *testing.T) {
	data := assessmentDataset(t)
	zero := scope.Scope{}
	data.Assignments[0].Scope = &zero
	_, err := rt.BuildSnapshot(data, time.Now())
	require.Error(t, err)
}

func TestAssignmentScopeFactsPreserveEachCompanyAndUnconfiguredAssignment(t *testing.T) {
	data := assessmentDataset(t)
	a, err := scope.New(1, scope.Stores, []meta.ID{10})
	require.NoError(t, err)
	b, err := scope.New(2, scope.AllStores, nil)
	require.NoError(t, err)
	data.Assignments = []rt.AssignmentRecord{{ID: 1, SubjectKey: "user:2", RoleID: 12, Scope: &a}, {ID: 2, SubjectKey: "user:2", RoleID: 12, Scope: &b}, {ID: 3, SubjectKey: "user:2", RoleID: 13}}
	snapshot, err := rt.BuildSnapshot(data, time.Now())
	require.NoError(t, err)
	sub, err := subject.ParseRef("user:2")
	require.NoError(t, err)
	result, err := snapshot.SubjectSnapshot(sub, "qs")
	require.NoError(t, err)
	require.Len(t, result.AssignmentFacts, 2)
	require.Len(t, result.AssignmentScopes, 3)
	require.Equal(t, "1", result.AssignmentScopes[0].AssignmentID)
	require.Equal(t, result.AssignmentScopes[0].Role.RoleID, result.AssignmentScopes[1].Role.RoleID)
	require.EqualValues(t, 1, result.AssignmentScopes[0].Scope.OrgID())
	require.EqualValues(t, 2, result.AssignmentScopes[1].Scope.OrgID())
	require.Nil(t, result.AssignmentScopes[2].Scope)
	*result.AssignmentScopes[0].Scope = b
	again, err := snapshot.SubjectSnapshot(sub, "qs")
	require.NoError(t, err)
	require.EqualValues(t, 1, again.AssignmentScopes[0].Scope.OrgID())
}
