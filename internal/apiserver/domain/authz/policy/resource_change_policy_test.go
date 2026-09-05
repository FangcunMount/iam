package policy_test

import (
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzschema"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestResourceChangePolicyRejectsInvalidCandidate(t *testing.T) {
	grant := assessmentRetryGrant(t, "tenant-a")
	candidate := documentResource(t, []string{"read"}, authzfixture.Schema())

	err := policy.ResourceChangePolicy{}.ValidateDependencies(candidate, []*permissiongrant.Grant{&grant})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrResourceInUse))
}

func TestResourceChangePolicyAcceptsCompatibleCandidate(t *testing.T) {
	grant := assessmentRetryGrant(t, "tenant-a")
	candidate := documentResource(t, []string{"retry", "read"}, authzfixture.Schema())

	require.NoError(t, policy.ResourceChangePolicy{}.ValidateDependencies(candidate, []*permissiongrant.Grant{&grant}))
}

func TestResourceChangePolicyAffectedTenantIDsIncludesAllActiveGrantTenants(t *testing.T) {
	grantA := assessmentRetryGrant(t, "tenant-a")
	grantB := assessmentRetryGrant(t, "tenant-b")
	revoked := assessmentRetryGrant(t, "tenant-c")
	require.NoError(t, revoked.Revoke(time.Now()))

	require.Equal(t, []string{"tenant-a", "tenant-b", "tenant-operator"}, policy.ResourceChangePolicy{}.AffectedResourceTenantIDs(
		"tenant-operator",
		[]*permissiongrant.Grant{&grantB, nil, &revoked, &grantA},
	))
}

func documentResource(t *testing.T, actions []string, schema attribute.Schema) resource.Resource {
	t.Helper()
	value, err := resource.NewResource(
		"example:catalog:collection:documents",
		actions,
		resource.WithID(resource.NewResourceID(101)),
		resource.WithDisplayName("Assessments"),
		resource.WithAttributeSchema(schema),
	)
	require.NoError(t, err)
	return value
}

func assessmentRetryGrant(t *testing.T, tenantID string) permissiongrant.Grant {
	t.Helper()
	origin := "active"
	constraints, err := constraint.New(constraint.Equal(authzfixture.AttributeKey, constraint.StringValue(origin)))
	require.NoError(t, err)
	grant, err := permissiongrant.New(
		meta.FromUint64(7),
		tenantID,
		resource.NewResourceID(101),
		"example:catalog:collection:documents",
		"retry",
		constraints,
		"operator",
	)
	require.NoError(t, err)
	grant.ID = meta.FromUint64(501)
	return grant
}
