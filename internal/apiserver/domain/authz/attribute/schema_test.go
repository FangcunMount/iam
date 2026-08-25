package attribute_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestAssessmentSchemaAllowsOnlyTrustedOriginValues(t *testing.T) {
	schema := attribute.AssessmentSchema()
	definition, ok := schema.Find(attribute.ObjectOriginType)
	require.True(t, ok)
	require.Equal(t, attribute.TypeString, definition.Type)
	require.Equal(t, []string{"adhoc", "plan"}, definition.AllowedStringValues)
}

func TestSchemaRejectsDuplicateAndInvalidDefinitions(t *testing.T) {
	_, err := attribute.NewSchema([]attribute.Definition{
		{Key: "object.origin_type", Type: attribute.TypeString},
		{Key: "object.origin_type", Type: attribute.TypeString},
	})
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = attribute.NewSchema([]attribute.Definition{{Key: "subject.department", Type: attribute.TypeString}})
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
