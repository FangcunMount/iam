package constraint_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestConstraintSetCanonicalizesAndValidatesAssessmentOrigin(t *testing.T) {
	set, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)
	require.NoError(t, set.ValidateAgainst(attribute.AssessmentSchema()))

	encoded, err := set.CanonicalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"version":1,"all_of":[{"key":"object.origin_type","operator":"eq","value":{"type":"string","string":"adhoc"}}]}`, string(encoded))

	parsed, err := constraint.ParseJSON(encoded)
	require.NoError(t, err)
	require.Equal(t, set, parsed)
}

func TestConstraintSetRejectsDuplicatesUnknownAttributesAndWrongTypes(t *testing.T) {
	_, err := constraint.New(
		constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")),
		constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("plan")),
	)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	unknown, err := constraint.New(constraint.Equal("object.department", constraint.StringValue("x")))
	require.NoError(t, err)
	err = unknown.ValidateAgainst(attribute.AssessmentSchema())
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	wrongType, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.Int64Value(1)))
	require.NoError(t, err)
	err = wrongType.ValidateAgainst(attribute.AssessmentSchema())
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestEmptyConstraintSetIsUnconditional(t *testing.T) {
	require.True(t, constraint.Empty().IsUnconditional())
}

func TestConstraintSetEvaluateAllOf(t *testing.T) {
	set, err := constraint.New(
		constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")),
		constraint.Equal("object.retryable", constraint.BoolValue(true)),
	)
	require.NoError(t, err)

	evaluation, err := set.Evaluate(constraint.Attributes{
		attribute.ObjectOriginType: constraint.StringValue("adhoc"),
		"object.retryable":         constraint.BoolValue(true),
	})
	require.NoError(t, err)
	require.True(t, evaluation.Matched)
	require.Empty(t, evaluation.MissingAttributeKeys)

	evaluation, err = set.Evaluate(constraint.Attributes{
		attribute.ObjectOriginType: constraint.StringValue("plan"),
		"object.retryable":         constraint.BoolValue(true),
	})
	require.NoError(t, err)
	require.False(t, evaluation.Matched)
}

func TestConstraintSetEvaluateMissingAttributeFailsClosed(t *testing.T) {
	set, err := constraint.New(
		constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")),
		constraint.Equal("object.retryable", constraint.BoolValue(true)),
	)
	require.NoError(t, err)

	evaluation, err := set.Evaluate(constraint.Attributes{})
	require.NoError(t, err)
	require.False(t, evaluation.Matched)
	require.Equal(t, []string{"object.origin_type", "object.retryable"}, evaluation.MissingAttributeKeys)
}

func TestConstraintSetEvaluateRejectsAttributeTypeMismatch(t *testing.T) {
	set, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)

	_, err = set.Evaluate(constraint.Attributes{
		attribute.ObjectOriginType: constraint.BoolValue(true),
	})
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
