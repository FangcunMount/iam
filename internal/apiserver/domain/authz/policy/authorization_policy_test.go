package policy

import (
	"testing"

	authz "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationPolicyGrantPermissionBuildsBusinessChange(t *testing.T) {
	t.Parallel()

	actor := mustActor(t, "operator-1")
	originScope := mustScope(t, authz.ScopeKindOrigin, "1")
	change, err := NewAuthorizationPolicy().GrantPermission(
		mustRole(t, "iam:origin_admin", "Origin Admin", "tenant-a"),
		mustResource(t,
			"iam:identity:collection:users",
			[]string{"update"},
			resource.WithScopeKinds([]authz.ScopeKind{authz.ScopeKindAll, authz.ScopeKindOrigin}),
		),
		"update",
		originScope,
		actor,
		"grant origin user update",
	)

	require.NoError(t, err)
	require.Equal(t, PolicyChangeGrantPermission, change.Kind)
	require.Equal(t, "tenant-a", change.TenantIDString())
	require.Equal(t, actor, change.Actor)
	require.NotNil(t, change.Permission)
	require.Equal(t, authz.Permission{
		RoleName:    "iam:origin_admin",
		TenantID:    "tenant-a",
		ResourceKey: "iam:identity:collection:users",
		Action:      "update",
		Scope:       originScope,
	}, *change.Permission)
}

func TestAuthorizationPolicyRejectsUnsupportedResourceActionOrScope(t *testing.T) {
	t.Parallel()

	actor := mustActor(t, "operator-1")
	originScope := mustScope(t, authz.ScopeKindOrigin, "1")
	catalog := mustResource(t, "iam:identity:collection:users", []string{"read"})

	_, err := NewAuthorizationPolicy().GrantPermission(
		mustRole(t, "iam:origin_admin", "Origin Admin", "tenant-a"),
		catalog,
		"update",
		authz.DefaultScope(),
		actor,
		"grant",
	)
	require.Error(t, err)

	_, err = NewAuthorizationPolicy().GrantPermission(
		mustRole(t, "iam:origin_admin", "Origin Admin", "tenant-a"),
		catalog,
		"read",
		originScope,
		actor,
		"grant",
	)
	require.Error(t, err)
}

func TestAuthorizationPolicyBindRoleBuildsBusinessChange(t *testing.T) {
	t.Parallel()

	actor := mustActor(t, "operator-1")
	subject, err := authz.NewSubject(authz.SubjectTypeUser, meta.FromUint64(100))
	require.NoError(t, err)

	change, err := NewAuthorizationPolicy().BindRole(
		subject,
		mustRole(t, "iam:admin", "Admin", "tenant-a"),
		actor,
		"bind admin",
	)

	require.NoError(t, err)
	require.Equal(t, PolicyChangeBindRole, change.Kind)
	require.Equal(t, "tenant-a", change.TenantIDString())
	require.NotNil(t, change.RoleBinding)
	require.Equal(t, authz.RoleBinding{
		Subject:   subject,
		RoleName:  "iam:admin",
		TenantID:  "tenant-a",
		GrantedBy: "operator-1",
	}, *change.RoleBinding)
}

func mustActor(t *testing.T, id string) Actor {
	t.Helper()
	actor, err := NewActor(id)
	require.NoError(t, err)
	return actor
}

func mustScope(t *testing.T, kind authz.ScopeKind, value string) authz.Scope {
	t.Helper()
	scope, err := authz.NewScope(kind, value)
	require.NoError(t, err)
	return scope
}

func mustRole(t *testing.T, name, displayName, tenantID string) role.Role {
	t.Helper()
	r, err := role.NewRole(name, displayName, tenantID)
	require.NoError(t, err)
	return r
}

func mustResource(t *testing.T, key string, actions []string, opts ...resource.ResourceOption) resource.Resource {
	t.Helper()
	r, err := resource.NewResource(key, actions, opts...)
	require.NoError(t, err)
	return r
}
