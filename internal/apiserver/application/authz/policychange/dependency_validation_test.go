package policychange_test

import (
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestValidateResourceDependenciesRejectsInvalidCandidate(t *testing.T) {
	grant := assessmentRetryGrant(t, "tenant-a")
	candidate := assessmentResource(t, []string{"read"}, attribute.AssessmentSchema())

	err := policychange.ValidateResourceDependencies(candidate, []*permissiongrant.Grant{&grant})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrResourceInUse))
}

func TestValidateResourceDependenciesAcceptsCompatibleCandidate(t *testing.T) {
	grant := assessmentRetryGrant(t, "tenant-a")
	candidate := assessmentResource(t, []string{"retry", "read"}, attribute.AssessmentSchema())

	require.NoError(t, policychange.ValidateResourceDependencies(candidate, []*permissiongrant.Grant{&grant}))
}

func TestValidateResourceDependenciesRejectsSchemaThatInvalidatesConstraint(t *testing.T) {
	grant := assessmentRetryGrant(t, "tenant-a")
	candidate := assessmentResource(t, []string{"retry"}, attribute.EmptySchema())

	err := policychange.ValidateResourceDependencies(candidate, []*permissiongrant.Grant{&grant})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrResourceInUse))
}

func TestEnsureRoleUnusedRejectsEveryActiveDependencyType(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policychange.EnsureRoleUnused(roleID, tt.assignments, tt.grants, tt.inheritances)
			require.Error(t, err)
			require.True(t, perrors.IsCode(err, code.ErrRoleInUse))
		})
	}
}

func TestAffectedResourceTenantIDsIncludesAllActiveGrantTenants(t *testing.T) {
	grantA := assessmentRetryGrant(t, "tenant-a")
	grantB := assessmentRetryGrant(t, "tenant-b")
	revoked := assessmentRetryGrant(t, "tenant-c")
	require.NoError(t, revoked.Revoke(time.Now()))

	require.Equal(t, []string{"tenant-a", "tenant-b", "tenant-operator"}, policychange.AffectedResourceTenantIDs(
		"tenant-operator",
		[]*permissiongrant.Grant{&grantB, nil, &revoked, &grantA},
	))
}

func assessmentResource(t *testing.T, actions []string, schema attribute.Schema) resource.Resource {
	t.Helper()
	value, err := resource.NewResource(
		"qs:evaluation:collection:assessments",
		actions,
		resource.WithID(resource.NewResourceID(101)),
		resource.WithAttributeSchema(schema),
	)
	require.NoError(t, err)
	return value
}

func assessmentRetryGrant(t *testing.T, tenantID string) permissiongrant.Grant {
	t.Helper()
	origin := "adhoc"
	constraints, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue(origin)))
	require.NoError(t, err)
	grant, err := permissiongrant.New(
		meta.FromUint64(7),
		tenantID,
		resource.NewResourceID(101),
		"qs:evaluation:collection:assessments",
		"retry",
		constraints,
		"operator",
	)
	require.NoError(t, err)
	grant.ID = meta.FromUint64(501)
	return grant
}
