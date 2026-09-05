package authorization_test

import (
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzschema"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestObjectContextRequiresIDForAttributes(t *testing.T) {
	_, err := authorization.NewObjectContext("", constraint.Attributes{
		authzfixture.AttributeKey: constraint.StringValue("active"),
	})
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestValidateAttributesUsesResourceSchema(t *testing.T) {
	require.NoError(t, authorization.ValidateAttributes(authzfixture.Schema(), constraint.Attributes{
		authzfixture.AttributeKey: constraint.StringValue("active"),
	}))

	err := authorization.ValidateAttributes(authzfixture.Schema(), constraint.Attributes{
		authzfixture.AttributeKey: constraint.StringValue("unknown"),
	})
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestDenyReportsMissingAttributesDeterministically(t *testing.T) {
	decision := authorization.Deny(7, []string{"object.z", "object.a", "object.z"}, nilTime())
	require.Equal(t, authorization.ReasonAttributeMissing, decision.Reason)
	require.Equal(t, []string{"object.a", "object.z"}, decision.MissingAttributeKeys)
	require.EqualValues(t, 7, decision.PolicyVersion)
}

func nilTime() (zeroTime time.Time) { return zeroTime }
