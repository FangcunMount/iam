package authorization_test

import (
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzschema"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

const documentResource = "example:catalog:collection:documents"

func TestEvaluatorAllowsMatchingCandidateAndPreservesEvidence(t *testing.T) {
	t.Parallel()

	request, catalogResource, roles, grantsByRole := evaluationFixture(t, "active")
	at := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)

	decision, err := authorization.NewEvaluator().Evaluate(request, authorization.EvaluationContext{
		EffectiveRoles: roles, GrantsByRole: grantsByRole, Resource: catalogResource, PolicyVersion: 9,
	}, at)

	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, meta.FromUint64(102), decision.MatchedGrantID)
	require.Equal(t, "example:evaluator", decision.MatchedRole)
	require.EqualValues(t, 9, decision.PolicyVersion)
	require.Equal(t, at, decision.EvaluatedAt)
}

func TestEvaluatorDeniesMissingAttributesWithDeterministicEvidence(t *testing.T) {
	t.Parallel()

	request, catalogResource, roles, grantsByRole := evaluationFixture(t, "")
	object, err := authorization.NewObjectContext("assessment-1", nil)
	require.NoError(t, err)
	request.Object = object

	decision, err := authorization.NewEvaluator().Evaluate(request, authorization.EvaluationContext{
		EffectiveRoles: roles, GrantsByRole: grantsByRole, Resource: catalogResource, PolicyVersion: 9,
	}, time.Time{})

	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.Equal(t, authorization.ReasonAttributeMissing, decision.Reason)
	require.Equal(t, []string{authzfixture.AttributeKey}, decision.MissingAttributeKeys)
}

func TestEvaluatorReturnsContractErrorForUnregisteredAttributes(t *testing.T) {
	t.Parallel()

	request, _, roles, grantsByRole := evaluationFixture(t, "active")

	decision, err := authorization.NewEvaluator().Evaluate(request, authorization.EvaluationContext{
		EffectiveRoles: roles, GrantsByRole: grantsByRole, PolicyVersion: 9,
	}, time.Time{})

	require.False(t, decision.Allowed)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestEvaluatorUsesCandidateOrderForMatchedEvidence(t *testing.T) {
	t.Parallel()

	request, catalogResource, roles, grantsByRole := evaluationFixture(t, "active")
	second := *grantsByRole[roles[0]][0]
	second.ID = meta.FromUint64(103)
	grantsByRole[roles[0]] = append(grantsByRole[roles[0]], &second)

	decision, err := authorization.NewEvaluator().Evaluate(request, authorization.EvaluationContext{
		EffectiveRoles: roles, GrantsByRole: grantsByRole, Resource: catalogResource, PolicyVersion: 9,
	}, time.Time{})

	require.NoError(t, err)
	require.Equal(t, meta.FromUint64(102), decision.MatchedGrantID)
	require.Equal(t, "example:evaluator", decision.MatchedRole)
}

func evaluationFixture(
	t testing.TB,
	statusValue string,
) (authorization.Request, *resource.Resource, []role.Name, map[role.Name][]*permissiongrant.Grant) {
	t.Helper()

	catalogResource, err := resource.NewResource(
		documentResource,
		[]string{"retry"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithDisplayName("Assessments"),
		resource.WithAttributeSchema(authzfixture.Schema()),
	)
	require.NoError(t, err)
	conditions, err := constraint.New(constraint.Equal(
		authzfixture.AttributeKey,
		constraint.StringValue("active"),
	))
	require.NoError(t, err)
	grant, err := permissiongrant.New(
		meta.FromUint64(12), "fangcun", catalogResource.ID,
		catalogResource.KeyString(), "retry", conditions, "bootstrap",
	)
	require.NoError(t, err)
	grant.ID = meta.FromUint64(102)
	roleName, err := role.NewName("example:evaluator")
	require.NoError(t, err)

	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)
	attributes := constraint.Attributes(nil)
	if statusValue != "" {
		attributes = constraint.Attributes{
			authzfixture.AttributeKey: constraint.StringValue(statusValue),
		}
	}
	object, err := authorization.NewObjectContext("assessment-1", attributes)
	require.NoError(t, err)
	request, err := authorization.NewRequest(sub, "fangcun", documentResource, "retry", object)
	require.NoError(t, err)

	return request, &catalogResource, []role.Name{roleName}, map[role.Name][]*permissiongrant.Grant{
		roleName: {&grant},
	}
}
