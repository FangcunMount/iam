package constraint_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzschema"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestConstraintSetCanonicalizesAndValidatesStatus(t *testing.T) {
	set, err := constraint.New(constraint.Equal(authzfixture.AttributeKey, constraint.StringValue("active")))
	require.NoError(t, err)
	require.NoError(t, set.ValidateAgainst(authzfixture.Schema()))

	encoded, err := set.CanonicalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"version":1,"all_of":[{"key":"object.status","operator":"eq","value":{"type":"string","string":"active"}}]}`, string(encoded))

	parsed, err := constraint.ParseJSON(encoded)
	require.NoError(t, err)
	require.Equal(t, set, parsed)
}

func TestConstraintSetRejectsDuplicatesUnknownAttributesAndWrongTypes(t *testing.T) {
	_, err := constraint.New(
		constraint.Equal(authzfixture.AttributeKey, constraint.StringValue("active")),
		constraint.Equal(authzfixture.AttributeKey, constraint.StringValue("paused")),
	)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	unknown, err := constraint.New(constraint.Equal("object.department", constraint.StringValue("x")))
	require.NoError(t, err)
	err = unknown.ValidateAgainst(authzfixture.Schema())
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	wrongType, err := constraint.New(constraint.Equal(authzfixture.AttributeKey, constraint.Int64Value(1)))
	require.NoError(t, err)
	err = wrongType.ValidateAgainst(authzfixture.Schema())
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestEmptyConstraintSetIsUnconditional(t *testing.T) {
	require.True(t, constraint.Empty().IsUnconditional())
}

func TestConstraintSetEvaluateAllOf(t *testing.T) {
	set, err := constraint.New(
		constraint.Equal(authzfixture.AttributeKey, constraint.StringValue("active")),
		constraint.Equal("object.retryable", constraint.BoolValue(true)),
	)
	require.NoError(t, err)

	evaluation, err := set.Evaluate(constraint.Attributes{
		authzfixture.AttributeKey: constraint.StringValue("active"),
		"object.retryable":        constraint.BoolValue(true),
	})
	require.NoError(t, err)
	require.True(t, evaluation.Matched)
	require.Empty(t, evaluation.MissingAttributeKeys)

	evaluation, err = set.Evaluate(constraint.Attributes{
		authzfixture.AttributeKey: constraint.StringValue("paused"),
		"object.retryable":        constraint.BoolValue(true),
	})
	require.NoError(t, err)
	require.False(t, evaluation.Matched)
}

func TestConstraintSetEvaluateMissingAttributeFailsClosed(t *testing.T) {
	set, err := constraint.New(
		constraint.Equal(authzfixture.AttributeKey, constraint.StringValue("active")),
		constraint.Equal("object.retryable", constraint.BoolValue(true)),
	)
	require.NoError(t, err)

	evaluation, err := set.Evaluate(constraint.Attributes{})
	require.NoError(t, err)
	require.False(t, evaluation.Matched)
	require.Equal(t, []string{"object.retryable", "object.status"}, evaluation.MissingAttributeKeys)
}

func TestConstraintSetEvaluateRejectsAttributeTypeMismatch(t *testing.T) {
	set, err := constraint.New(constraint.Equal(authzfixture.AttributeKey, constraint.StringValue("active")))
	require.NoError(t, err)

	_, err = set.Evaluate(constraint.Attributes{
		authzfixture.AttributeKey: constraint.BoolValue(true),
	})
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
