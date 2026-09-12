package authz

import (
	pb "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScopedSnapshotCompatibilityAndMalformedData(t *testing.T) {
	s := &pb.GetAuthorizationSnapshotResponse{PolicyVersion: 1}
	require.Error(t, ValidateScopedSnapshot(s))
	s.ScopeContractVersion = 1
	require.NoError(t, ValidateScopedSnapshot(s))
	entry := &pb.PermissionEntry{Resource: "qs:evaluation:collection:assessments", Action: "retry", Mode: pb.AuthorizationMode_UNCONDITIONAL}
	s.Permissions = []*pb.PermissionEntry{entry}
	require.NoError(t, ValidateScopedSnapshot(s))
	entry.Scopes = []*pb.DataScope{{OrgId: "1", Kind: pb.DataScopeKind_STORES, StoreIds: []string{"123456789012345678"}}}
	require.NoError(t, ValidateScopedSnapshot(s))
	for _, bad := range []*pb.DataScope{nil, {OrgId: "0", Kind: pb.DataScopeKind_ALL_STORES}, {OrgId: "1", Kind: pb.DataScopeKind_STORES}, {OrgId: "1", Kind: 99}, {OrgId: "1", Kind: pb.DataScopeKind_ALL_STORES, StoreIds: []string{"1"}}, {OrgId: "1", Kind: pb.DataScopeKind_STORES, StoreIds: []string{"9223372036854775808"}}} {
		entry.Scopes = []*pb.DataScope{bad}
		require.Error(t, ValidateScopedSnapshot(s))
	}
}

func TestAssignmentScopeFactsMustBeCompleteAndConsistent(t *testing.T) {
	role := &pb.AssignmentRoleFact{RoleId: "1", RoleName: "qs:assessment_operator", ManagementProtection: "standard"}
	s := &pb.GetAuthorizationSnapshotResponse{PolicyVersion: 1, ScopeContractVersion: 1, AssignmentFactsComplete: true, AssignmentFacts: []*pb.AssignmentRoleFact{role}}
	require.Error(t, ValidateAssignmentScopes(s), "missing per-assignment facts must not mean empty role set")
	first := &pb.AssignmentScopeFact{AssignmentId: "10", Role: role, Scope: &pb.DataScope{OrgId: "1", Kind: pb.DataScopeKind_STORES, StoreIds: []string{"7"}}}
	second := &pb.AssignmentScopeFact{AssignmentId: "11", Role: role, Scope: &pb.DataScope{OrgId: "2", Kind: pb.DataScopeKind_ALL_STORES}}
	s.AssignmentScopes = []*pb.AssignmentScopeFact{first, second}
	require.NoError(t, ValidateAssignmentScopes(s))
	second.Scope = nil
	require.NoError(t, ValidateAssignmentScopes(s), "unconfigured scope remains explicit")
	second.AssignmentId = "10"
	require.Error(t, ValidateAssignmentScopes(s))
	second.AssignmentId = "11"
	second.Role = &pb.AssignmentRoleFact{RoleId: "1", RoleName: "platform_admin"}
	require.Error(t, ValidateAssignmentScopes(s))
	second.Role = role
	first.Scope.StoreIds = nil
	require.Error(t, ValidateAssignmentScopes(s))
	first.Scope.StoreIds = []string{"7"}
	s.AssignmentFactsComplete = false
	require.Error(t, ValidateAssignmentScopes(s))
}
