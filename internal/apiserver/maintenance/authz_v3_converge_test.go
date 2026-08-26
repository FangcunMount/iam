package maintenance

import (
	"testing"

	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	bindingrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestFindMissingSubjectRequiresExactFingerprintAndFourRoles(t *testing.T) {
	roles := make([]*rolerepo.RolePO, 0, len(missingSubjectRoles))
	state := &convergeState{users: map[string]convergeUserState{
		"100": {Active: true},
	}}
	for index, name := range missingSubjectRoles {
		id := meta.FromUint64(uint64(index + 1))
		roles = append(roles, &rolerepo.RolePO{
			AuditFields: mysql.AuditFields{ID: id},
			TenantID:    "fangcun",
			Name:        name,
		})
		state.assignments = append(state.assignments,
			&bindingrepo.BindingPO{RoleID: id.Uint64(), TenantID: "fangcun", SubjectType: "user", SubjectID: "100"},
			&bindingrepo.BindingPO{RoleID: id.Uint64(), TenantID: "fangcun", SubjectType: "user", SubjectID: "999"},
		)
	}
	state.roles = roles

	plan := &AuthzV3ConvergePlan{}
	findMissingSubjectWithFingerprint(state, plan, fingerprintSubject("999"))
	require.Empty(t, plan.Summary.Blockers)
	require.Equal(t, "999", plan.missingSubjectID)
	require.Equal(t, []string{fingerprintSubject("999")}, plan.Summary.MissingSubjectFingerprints)
}

func TestFindMissingSubjectBlocksUnexpectedInactiveAssignments(t *testing.T) {
	state := &convergeState{users: map[string]convergeUserState{}}
	for index, name := range missingSubjectRoles {
		id := meta.FromUint64(uint64(index + 1))
		state.roles = append(state.roles, &rolerepo.RolePO{
			AuditFields: mysql.AuditFields{ID: id}, TenantID: "fangcun", Name: name,
		})
		state.assignments = append(state.assignments, &bindingrepo.BindingPO{
			RoleID: id.Uint64(), TenantID: "fangcun", SubjectType: "user", SubjectID: "999",
		})
	}
	state.assignments = append(state.assignments, &bindingrepo.BindingPO{
		RoleID: 1, TenantID: "fangcun", SubjectType: "user", SubjectID: "unexpected",
	})

	plan := &AuthzV3ConvergePlan{}
	findMissingSubjectWithFingerprint(state, plan, fingerprintSubject("999"))
	require.Contains(t, plan.Summary.Blockers, "unexpected_missing_or_inactive_user_assignments")
}

func TestConvergedCountsRequireOnlyApprovedIntermediateOrFinal(t *testing.T) {
	require.Len(t, targetRoleKeys, targetCounts.Roles)
	require.Len(t, targetResourceKeys, targetCounts.Resources)
	require.Len(t, append(append([]inheritanceSpec(nil), sourceInheritances...), targetInheritanceAdditions...), targetCounts.Inheritances)
	require.True(t, isCatalogConvergedCounts(AuthzV3Counts{Roles: 9, Resources: 27, Assignments: 133, Inheritances: 8, Grants: 100}))
	require.True(t, isCatalogConvergedCounts(targetCounts))
	require.False(t, isCatalogConvergedCounts(AuthzV3Counts{Roles: 9, Resources: 27, Assignments: 134, Inheritances: 8, Grants: 100}))
}

func TestGrantReplacementManifestDoesNotRevokeUnchangedGrants(t *testing.T) {
	removals := make(map[grantSpec]struct{}, len(sourceGrantRemovals))
	for _, spec := range sourceGrantRemovals {
		removals[spec] = struct{}{}
	}
	for _, spec := range targetGrantAdditions {
		_, duplicated := removals[spec]
		require.False(t, duplicated, "unchanged grant must not be both revoked and recreated: %+v", spec)
	}
	require.Len(t, sourceGrantRemovals, 12)
	require.Len(t, targetGrantAdditions, 7)
	require.Equal(t, sourceCounts.Grants-len(sourceGrantRemovals)+len(targetGrantAdditions), targetCounts.Grants)
}

func TestStateHashDoesNotExposeSubjectAndIsOrderIndependent(t *testing.T) {
	roleID := meta.FromUint64(1)
	makeState := func(subjects ...string) *convergeState {
		state := &convergeState{
			roles: []*rolerepo.RolePO{{
				AuditFields: mysql.AuditFields{ID: roleID}, TenantID: "fangcun", Name: "qs:staff",
			}},
			users: map[string]convergeUserState{},
		}
		for _, subjectID := range subjects {
			state.assignments = append(state.assignments, &bindingrepo.BindingPO{
				TenantID: "fangcun", SubjectType: "user", SubjectID: subjectID, RoleID: roleID.Uint64(),
			})
		}
		return state
	}
	left, err := hashConvergeState(makeState("100", "200"))
	require.NoError(t, err)
	right, err := hashConvergeState(makeState("200", "100"))
	require.NoError(t, err)
	require.Equal(t, left, right)
	require.NotContains(t, left, "100")
	require.Len(t, left, 64)
}
