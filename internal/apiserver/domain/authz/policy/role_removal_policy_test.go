package policy_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRoleRemovalPolicyRejectsEveryActiveDependencyType(t *testing.T) {
	roleID := meta.FromUint64(11)
	assigned, err := assignment.NewAssignment(assignment.SubjectTypeUser, meta.FromUint64(22), roleID, "tenant-a", assignment.WithGrantedBy("operator"))
	require.NoError(t, err)
	grant := assessmentRetryGrant(t, "tenant-a")
	grant.RoleID = roleID
	inheritance, err := roleinheritance.New(roleID, meta.FromUint64(33), "tenant-a", "operator")
	require.NoError(t, err)

	tests := []struct {
		name         string
		assignments  []*assignment.Assignment
		grants       []*permissiongrant.Grant
		inheritances []*roleinheritance.Inheritance
	}{
		{name: "assignment", assignments: []*assignment.Assignment{&assigned}},
		{name: "grant", grants: []*permissiongrant.Grant{&grant}},
		{name: "inheritance", inheritances: []*roleinheritance.Inheritance{&inheritance}},
	}
	p := policy.RoleRemovalPolicy{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.EnsureUnused(roleID, tt.assignments, tt.grants, tt.inheritances)
			require.Error(t, err)
			require.True(t, perrors.IsCode(err, code.ErrRoleInUse))
		})
	}
}
