package identity

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestIdentityGRPCRuntimeRegistersOnlyImplementedServices(t *testing.T) {
	server := grpc.NewServer()
	NewService(nil, nil, nil, nil, nil, nil, nil, nil).Register(server)

	info := server.GetServiceInfo()
	require.Contains(t, info, "iam.identity.v2.IdentityRead")
	require.Contains(t, info, "iam.identity.v2.ProfileLinkQuery")
	require.Contains(t, info, "iam.identity.v2.ProfileLinkCommand")
	require.Contains(t, info, "iam.identity.v2.IdentityLifecycle")
	assert.NotContains(t, info, "iam.identity.v2.IdentityStream")

	assert.ElementsMatch(t, []string{
		"GetUser", "BatchGetUsers", "SearchUsers", "GetProfile", "BatchGetProfiles",
	}, methodNames(info["iam.identity.v2.IdentityRead"]))
	assert.ElementsMatch(t, []string{
		"HasProfileLink", "ListProfiles", "ListProfileLinks",
	}, methodNames(info["iam.identity.v2.ProfileLinkQuery"]))
	assert.ElementsMatch(t, []string{
		"EstablishProfileLink", "RevokeProfileLink", "BatchRevokeProfileLinks", "ImportProfileLinks",
	}, methodNames(info["iam.identity.v2.ProfileLinkCommand"]))
	assert.ElementsMatch(t, []string{
		"CreateUser", "UpdateUser", "DeactivateUser", "BlockUser",
	}, methodNames(info["iam.identity.v2.IdentityLifecycle"]))
}

func TestIdentityContractsDoNotReferenceRemovedRPCs(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		filepath.Join(root, "api/grpc/README.md"),
		filepath.Join(root, "configs/grpc_acl.yaml"),
		filepath.Join(root, "pkg/sdk/identity/client.go"),
		filepath.Join(root, "pkg/sdk/identity/profile_link.go"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(data)
		assert.NotContains(t, content, "UpdateProfileLinkRelation", path)
		assert.NotContains(t, content, "LinkExternalIdentity", path)
		assert.NotContains(t, content, "IdentityStream", path)
	}
}

func methodNames(info grpc.ServiceInfo) []string {
	names := make([]string, 0, len(info.Methods))
	for _, method := range info.Methods {
		names = append(names, method.Name)
	}
	return names
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../../"))
}
