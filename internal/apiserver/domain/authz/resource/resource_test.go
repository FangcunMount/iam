package resource

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestResourceOwnsActionsAndVersionedAttributeSchema(t *testing.T) {
	resource, err := NewResource(
		"qs:evaluation:collection:assessments",
		[]string{"read", "retry", "force_retry"},
		WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)
	require.True(t, resource.HasAction("retry"))
	require.False(t, resource.HasAction("delete"))
	definition, ok := resource.AttributeSchema.Find(attribute.ObjectOriginType)
	require.True(t, ok)
	require.Equal(t, []string{"adhoc", "plan"}, definition.AllowedStringValues)

	require.NoError(t, resource.ChangeCatalog([]string{"read", "retry"}))
	require.False(t, resource.HasAction("force_retry"))
	require.True(t, perrors.IsCode(resource.ChangeCatalog(nil), code.ErrInvalidArgument))
}

func TestResourceSeparatesExactKeysFromTrustedPatterns(t *testing.T) {
	key, err := NewKey("scale:form:template:*")
	require.NoError(t, err)
	require.Equal(t, "scale:form:template:*", key.String())
	_, err = NewKey("qs:*:*:*")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	pattern, err := NewPattern("qs:*:*:*")
	require.NoError(t, err)
	assessment, err := NewPattern("qs:evaluation:collection:assessments")
	require.NoError(t, err)
	require.True(t, pattern.Covers(assessment))
}

func TestConcreteActionRejectsLegacyExpressions(t *testing.T) {
	for _, value := range []string{"read|list", ".*", "*"} {
		_, err := NewAction(value)
		require.True(t, perrors.IsCode(err, code.ErrInvalidArgument), value)
	}
}
