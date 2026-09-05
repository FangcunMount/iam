package attributeproviders_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/attributeproviders"
	"github.com/stretchr/testify/require"
)

func TestConfiguredTrustIsExactAndIndependentOfResourceDeployment(t *testing.T) {
	policy, err := attributeproviders.Load("../../../../../configs/authz_attribute_providers.yaml")
	require.NoError(t, err)
	request := objectattributeadmission.Request{CallerService: "qs-apiserver.svc", ResourceKey: "qs:evaluation:collection:assessments", AttributeKey: "object.origin_type"}
	require.NoError(t, policy.AuthorizeAttribute(request))
	request.CallerService = "unknown.svc"
	require.Error(t, policy.AuthorizeAttribute(request))
	config := `providers:
  - service: example.svc
    resource: example:catalog:collection:documents
    attributes: [object.status]
`
	path := filepath.Join(t.TempDir(), "providers.yaml")
	require.NoError(t, os.WriteFile(path, []byte(config), 0600))
	policy, err = attributeproviders.Load(path)
	require.NoError(t, err)
	conditions, err := constraint.New(constraint.Equal("object.status", constraint.StringValue("ready")))
	require.NoError(t, err)
	require.NoError(t, policy.ValidateCoverage("example:catalog:collection:documents", conditions))
	require.Error(t, policy.ValidateCoverage("other", conditions))
	require.NoError(t, policy.ValidateCoverage("other", constraint.Empty()))
}
func TestInvalidProviderFilesFailClosed(t *testing.T) {
	_, err := attributeproviders.Load("missing.yaml")
	require.Error(t, err)
	for _, content := range []string{"", "null", "{}", "providers: [", "unknown: true", "providers: []\n---\nproviders: []", "providers: [{service: '*', resource: x, attributes: [object.status]}]", "providers: [{service: x, resource: x, attributes: ['*']}]"} {
		path := filepath.Join(t.TempDir(), "providers.yaml")
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		_, err := attributeproviders.Load(path)
		require.Error(t, err, content)
	}
}
