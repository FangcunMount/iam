package attribute_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzschema"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestSchemaAllowsOnlyDeclaredStatusValues(t *testing.T) {
	schema := authzfixture.Schema()
	definition, ok := schema.Find(authzfixture.AttributeKey)
	require.True(t, ok)
	require.Equal(t, attribute.TypeString, definition.Type)
	require.Equal(t, []string{"active", "paused"}, definition.AllowedStringValues)
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
