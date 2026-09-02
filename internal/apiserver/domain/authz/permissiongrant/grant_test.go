package permissiongrant_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestManagedGrantUsesStableCanonicalKey(t *testing.T) {
	constraints, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)
	grant, err := permissiongrant.New(
		meta.FromUint64(10),
		"tenant-a",
		resource.NewResourceID(20),
		"qs:evaluation:collection:assessments",
		"retry",
		constraints,
		"operator-1",
	)
	require.NoError(t, err)
	require.True(t, grant.IsConditional())
	require.Len(t, grant.GrantKey, 64)

	second, err := permissiongrant.New(
		meta.FromUint64(10), "tenant-a", resource.NewResourceID(20),
		"qs:evaluation:collection:assessments", "retry", constraints, "operator-2",
	)
	require.NoError(t, err)
	require.Equal(t, grant.GrantKey, second.GrantKey)
}

func TestManagedGrantRequiresCatalogResourceAndConcreteAction(t *testing.T) {
	_, err := permissiongrant.New(
		meta.FromUint64(10), "tenant-a", resource.ResourceID{},
		"qs:*:*:*", "retry", constraint.Empty(), "operator-1",
	)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = permissiongrant.New(
		meta.FromUint64(10), "tenant-a", resource.NewResourceID(20),
		"qs:evaluation:collection:assessments", "*", constraint.Empty(), "operator-1",
	)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestSystemWildcardGrantMustBeUnconditional(t *testing.T) {
	conditional, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)
	_, err = permissiongrant.NewSystem(
		meta.FromUint64(10), "tenant-a", resource.ResourceID{},
		"*:*:*:*", permissiongrant.WildcardAction, conditional, "bootstrap",
	)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	grant, err := permissiongrant.NewSystem(
		meta.FromUint64(10), "tenant-a", resource.ResourceID{},
		"*:*:*:*", permissiongrant.WildcardAction, constraint.Empty(), "bootstrap",
	)
	require.NoError(t, err)
	action, err := resource.NewAction("retry")
	require.NoError(t, err)
	require.True(t, grant.MatchesAction(action))
}

func TestManagedGrantValidatesAgainstResourceSchema(t *testing.T) {
	constraints, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)
	grant, err := permissiongrant.New(
		meta.FromUint64(10), "tenant-a", resource.NewResourceID(20),
		"qs:evaluation:collection:assessments", "retry", constraints, "operator-1",
	)
	require.NoError(t, err)
	catalogResource, err := resource.NewResource(
		"qs:evaluation:collection:assessments",
		[]string{"retry", "batch_evaluate"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithDisplayName("Assessments"),
		resource.WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)
	require.NoError(t, grant.ValidateAgainst(catalogResource))

	invalid, err := permissiongrant.New(
		meta.FromUint64(10), "tenant-a", resource.NewResourceID(20),
		"qs:evaluation:collection:assessments", "retry",
		constraint.Empty(), "operator-1",
	)
	require.NoError(t, err)
	otherResource, err := resource.NewResource(
		"qs:evaluation:collection:other", []string{"retry"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithDisplayName("Other"),
	)
	require.NoError(t, err)
	err = invalid.ValidateAgainst(otherResource)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestConditionalGrantCannotAuthorizeCollectionOrBatchActions(t *testing.T) {
	constraints, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)
	for _, action := range []string{"list", "search", "batch_evaluate"} {
		_, err := permissiongrant.New(
			meta.FromUint64(10), "tenant-a", resource.NewResourceID(20),
			"qs:evaluation:collection:assessments", action, constraints, "operator-1",
		)
		require.True(t, perrors.IsCode(err, code.ErrInvalidArgument), action)
	}
}
