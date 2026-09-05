package objectattributeadmission_test

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/assessment"
	"github.com/stretchr/testify/require"
)

func TestDefaultPolicyAllowsTrustedAssessmentOrigin(t *testing.T) {
	policy := authzfixture.Policy()
	require.NoError(t, policy.AuthorizeAttribute(objectattributeadmission.Request{
		CallerService: authzfixture.Service,
		ResourceKey:   authzfixture.Resource,
		AttributeKey:  authzfixture.AttributeKey,
	}))
}

func TestDefaultPolicyRejectsUntrustedCaller(t *testing.T) {
	policy := authzfixture.Policy()
	err := policy.AuthorizeAttribute(objectattributeadmission.Request{
		CallerService: "other.svc",
		ResourceKey:   authzfixture.Resource,
		AttributeKey:  authzfixture.AttributeKey,
	})
	require.ErrorAs(t, err, new(objectattributeadmission.ErrUntrustedCaller))
}

func TestDefaultPolicyRejectsUnsupportedAttribute(t *testing.T) {
	policy := authzfixture.Policy()
	err := policy.AuthorizeAttribute(objectattributeadmission.Request{
		CallerService: authzfixture.Service,
		ResourceKey:   authzfixture.Resource,
		AttributeKey:  "object.other",
	})
	require.ErrorAs(t, err, new(objectattributeadmission.ErrUnsupportedAttribute))
}
