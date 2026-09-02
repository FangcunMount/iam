package authorization_test

import (
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

const assessmentResource = "qs:evaluation:collection:assessments"

func TestEvaluatorAllowsMatchingCandidateAndPreservesEvidence(t *testing.T) {
	t.Parallel()

	request, catalogResource, roles, grantsByRole := evaluationFixture(t, "adhoc")
	at := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)

	decision, err := authorization.NewEvaluator().Evaluate(request, authorization.EvaluationContext{
		EffectiveRoles: roles, GrantsByRole: grantsByRole, Resource: catalogResource, PolicyVersion: 9,
	}, at)

	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, meta.FromUint64(102), decision.MatchedGrantID)
	require.Equal(t, "qs:evaluator", decision.MatchedRole)
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
	require.Equal(t, []string{attribute.ObjectOriginType}, decision.MissingAttributeKeys)
}

func TestEvaluatorReturnsContractErrorForUnregisteredAttributes(t *testing.T) {
	t.Parallel()

	request, _, roles, grantsByRole := evaluationFixture(t, "adhoc")

	decision, err := authorization.NewEvaluator().Evaluate(request, authorization.EvaluationContext{
		EffectiveRoles: roles, GrantsByRole: grantsByRole, PolicyVersion: 9,
	}, time.Time{})

	require.False(t, decision.Allowed)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestEvaluatorUsesCandidateOrderForMatchedEvidence(t *testing.T) {
	t.Parallel()

	request, catalogResource, roles, grantsByRole := evaluationFixture(t, "adhoc")
	second := *grantsByRole[roles[0]][0]
	second.ID = meta.FromUint64(103)
	grantsByRole[roles[0]] = append(grantsByRole[roles[0]], &second)

	decision, err := authorization.NewEvaluator().Evaluate(request, authorization.EvaluationContext{
		EffectiveRoles: roles, GrantsByRole: grantsByRole, Resource: catalogResource, PolicyVersion: 9,
	}, time.Time{})

	require.NoError(t, err)
	require.Equal(t, meta.FromUint64(102), decision.MatchedGrantID)
	require.Equal(t, "qs:evaluator", decision.MatchedRole)
}

func evaluationFixture(
	t testing.TB,
	originType string,
) (authorization.Request, *resource.Resource, []role.Name, map[role.Name][]*permissiongrant.Grant) {
	t.Helper()

	catalogResource, err := resource.NewResource(
		assessmentResource,
		[]string{"retry"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithDisplayName("Assessments"),
		resource.WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)
	conditions, err := constraint.New(constraint.Equal(
		attribute.ObjectOriginType,
		constraint.StringValue("adhoc"),
	))
	require.NoError(t, err)
	grant, err := permissiongrant.New(
		meta.FromUint64(12), "fangcun", catalogResource.ID,
		catalogResource.KeyString(), "retry", conditions, "bootstrap",
	)
	require.NoError(t, err)
	grant.ID = meta.FromUint64(102)
	roleName, err := role.NewName("qs:evaluator")
	require.NoError(t, err)

	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)
	attributes := constraint.Attributes(nil)
	if originType != "" {
		attributes = constraint.Attributes{
			attribute.ObjectOriginType: constraint.StringValue(originType),
		}
	}
	object, err := authorization.NewObjectContext("assessment-1", attributes)
	require.NoError(t, err)
	request, err := authorization.NewRequest(sub, "fangcun", assessmentResource, "retry", object)
	require.NoError(t, err)

	return request, &catalogResource, []role.Name{roleName}, map[role.Name][]*permissiongrant.Grant{
		roleName: {&grant},
	}
}
