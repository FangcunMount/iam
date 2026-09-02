package objectattributeadmission_test

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/stretchr/testify/require"
)

func TestDefaultPolicyAllowsTrustedAssessmentOrigin(t *testing.T) {
	policy := objectattributeadmission.NewDefaultPolicy()
	require.NoError(t, policy.AuthorizeAttribute(objectattributeadmission.Request{
		CallerService: objectattributeadmission.TrustedAssessmentAttributeService,
		ResourceKey:   objectattributeadmission.AssessmentResource,
		AttributeKey:  attribute.ObjectOriginType,
	}))
}

func TestDefaultPolicyRejectsUntrustedCaller(t *testing.T) {
	policy := objectattributeadmission.NewDefaultPolicy()
	err := policy.AuthorizeAttribute(objectattributeadmission.Request{
		CallerService: "other.svc",
		ResourceKey:   objectattributeadmission.AssessmentResource,
		AttributeKey:  attribute.ObjectOriginType,
	})
	require.ErrorAs(t, err, new(objectattributeadmission.ErrUntrustedCaller))
}

func TestDefaultPolicyRejectsUnsupportedAttribute(t *testing.T) {
	policy := objectattributeadmission.NewDefaultPolicy()
	err := policy.AuthorizeAttribute(objectattributeadmission.Request{
		CallerService: objectattributeadmission.TrustedAssessmentAttributeService,
		ResourceKey:   objectattributeadmission.AssessmentResource,
		AttributeKey:  "object.other",
	})
	require.ErrorAs(t, err, new(objectattributeadmission.ErrUnsupportedAttribute))
}
