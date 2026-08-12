package permission

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestPermissionUsesTypedBusinessFactsWithStringAccessors(t *testing.T) {
	t.Parallel()

	permission, err := New(" iam:admin ", " tenant-a ", " qs:*:*:* ", " read|list ")
	require.NoError(t, err)
	require.Equal(t, "iam:admin", permission.RoleNameString())
	require.Equal(t, "tenant-a", permission.TenantIDString())
	require.Equal(t, "qs:*:*:*", permission.ResourceKeyString())
	require.Equal(t, "read|list", permission.ActionString())
	require.Equal(t, scope.Default(), permission.Scope)

	wildcard, err := New("super_admin", "platform", "*:*:*:*", ".*")
	require.NoError(t, err)
	require.Equal(t, "*:*:*:*", wildcard.ResourceKeyString())
	require.Equal(t, ".*", wildcard.ActionString())
}

func TestPermissionRejectsInvalidBusinessFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		roleName    string
		tenantID    string
		resourceKey string
		action      string
		scope       scope.Scope
	}{
		{name: "missing role", tenantID: "tenant-a", resourceKey: "iam:identity:collection:users", action: "read", scope: scope.Default()},
		{name: "missing tenant", roleName: "iam:admin", resourceKey: "iam:identity:collection:users", action: "read", scope: scope.Default()},
		{name: "legacy resource", roleName: "iam:admin", tenantID: "tenant-a", resourceKey: "iam:users", action: "read", scope: scope.Default()},
		{name: "missing action", roleName: "iam:admin", tenantID: "tenant-a", resourceKey: "iam:identity:collection:users", scope: scope.Default()},
		{name: "invalid scope", roleName: "iam:admin", tenantID: "tenant-a", resourceKey: "iam:identity:collection:users", action: "read", scope: scope.Scope{Kind: scope.KindAll, Value: "1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.roleName, tt.tenantID, tt.resourceKey, tt.action, WithScope(tt.scope))
			require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
		})
	}
}
