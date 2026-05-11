package policy

import (
	"context"
	"errors"
	"testing"

	authzuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/apiserver/eventing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyChangeCommitterCommitsPermissionAndKeepsFactsWhenReloadFails(t *testing.T) {
	permission := mustCommitterPermission(t, "iam:admin", "tenant-a", "iam:identity:collection:users", "read")
	actor := mustPolicyActor(t, "operator-1")
	versionRepo := &policyVersionRepoForCommandStub{}
	facts := &policyAuthorizationFactStoreStub{}
	stager := &policyEventStagerStub{}
	runtime := &policyCasbinAdapterStub{loadErr: errors.New("reload failed")}

	committer := NewPolicyChangeCommitter(&policyUowStub{tx: authzuow.TxRepositories{
		PolicyVersions:     versionRepo,
		AuthorizationFacts: facts,
		Events:             stager,
	}}, runtime)

	err := committer.Commit(context.Background(), func(context.Context, authzuow.TxRepositories) (policyDomain.PolicyChange, error) {
		return policyDomain.PolicyChange{
			Kind:       policyDomain.PolicyChangeGrantPermission,
			TenantID:   "tenant-a",
			Actor:      actor,
			Reason:     "grant read",
			Permission: &permission,
		}, nil
	})

	require.NoError(t, err)
	require.Len(t, facts.policyAdds, 1)
	assert.Equal(t, permission, facts.policyAdds[0])
	assert.Equal(t, 1, versionRepo.incrementCalls)
	require.Len(t, stager.events, 1)
	assert.Equal(t, eventing.AuthzVersionChanged, stager.events[0].EventType())
	assert.Equal(t, 3, runtime.loadCalls)
}

func mustPolicyActor(t *testing.T, id string) policyDomain.Actor {
	t.Helper()
	actor, err := policyDomain.NewActor(id)
	require.NoError(t, err)
	return actor
}

func mustCommitterPermission(t *testing.T, roleName, tenantID, resourceKey, action string) permission.Permission {
	t.Helper()
	permission, err := permission.New(roleName, tenantID, resourceKey, action)
	require.NoError(t, err)
	return permission
}
