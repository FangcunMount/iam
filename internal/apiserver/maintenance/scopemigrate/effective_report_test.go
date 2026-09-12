package scopemigrate

import (
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	resourcepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEffectiveReportUnionsOnlyMatchingActionAndCompany(t *testing.T) {
	state, _, _ := fixture()
	secondRole := state.Roles[0]
	secondRole.ID = 6
	secondRole.Name = "qs:result_reviewer"
	state.Roles = append(state.Roles, secondRole)
	second := state.Assignments[0]
	second.ID = 5
	second.RoleID = 6
	state.Assignments = append(state.Assignments, second)
	catalog := resourcepo.ResourcePO{Key: "qs:actor:collection:testees", Actions: `["read","list"]`}
	catalog.ID = 4
	state.Resources = []resourcepo.ResourcePO{catalog}
	grant, err := domain.New(2, resource.NewResourceID(4), catalog.Key, "read", "user:9")
	require.NoError(t, err)
	grant.ID = 10
	row, err := (grantpo.Mapper{}).ToPO(&grant)
	require.NoError(t, err)
	state.Grants = append(state.Grants, *row)
	grant, err = domain.NewSystem(6, resource.NewResourceID(0), "qs:*:*:*", "*", "user:9")
	require.NoError(t, err)
	grant.ID = 11
	row, err = (grantpo.Mapper{}).ToPO(&grant)
	require.NoError(t, err)
	state.Grants = append(state.Grants, *row)
	changes := []Change{{AssignmentID: "1", Targets: []Target{{OrgID: "7", Kind: scope.Stores, StoreIDs: []string{"8"}}}}, {AssignmentID: "5", Targets: []Target{{OrgID: "7", Kind: scope.Stores, StoreIDs: []string{"9"}}, {OrgID: "8", Kind: scope.AllStores, StoreIDs: []string{}}}}}
	rows, err := effectivePermissions(state, changes)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	for _, r := range rows {
		if r.OrgID == "8" {
			require.Equal(t, scope.AllStores, r.Kind)
			require.Empty(t, r.StoreIDs)
			continue
		}
		if r.Action == "read" {
			require.Equal(t, []string{"8", "9"}, r.StoreIDs)
			require.Equal(t, []string{"1", "5"}, r.AssignmentIDs)
		} else {
			require.Equal(t, []string{"9"}, r.StoreIDs)
		}
	}
	changes[1].Targets[0].Kind = scope.AllStores
	changes[1].Targets[0].StoreIDs = []string{}
	rows, err = effectivePermissions(state, changes)
	require.NoError(t, err)
	for _, r := range rows {
		require.Equal(t, scope.AllStores, r.Kind)
		require.Empty(t, r.StoreIDs)
	}
}
