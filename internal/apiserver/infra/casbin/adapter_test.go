package casbin

import (
	"context"
	"testing"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCasbinAdapterEnforcesMemoryPolicyFacts(t *testing.T) {
	t.Parallel()

	adapter := setupCasbinAdapter(t)
	ctx := context.Background()

	require.NoError(t, adapter.addGroupingFacts(ctx, GroupingRule{Sub: "user:100", Dom: "tenant-a", Role: "role:iam:admin"}))
	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:iam:admin", Dom: "tenant-a", Obj: "iam:user:*", Act: "read"}))

	allowed, err := adapter.AuthorizeRoute(ctx, "user:100", "tenant-a", "iam:user:*", "read")
	require.NoError(t, err)
	require.True(t, allowed)

	roles, err := adapter.implicitRolesForUser(ctx, "user:100", "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []string{"role:iam:admin"}, roles)

	permissions, err := adapter.implicitPermissionsForUser(ctx, "user:100", "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []PolicyRule{
		{Sub: "role:iam:admin", Dom: "tenant-a", Obj: "iam:user:*", Act: "read", Scope: "all:*"},
	}, permissions)

	subject, err := authzDomain.NewSubject(authzDomain.SubjectTypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	decision, err := adapter.Check(ctx, mustAuthorizationRequest(t, subject, "tenant-a", "iam:user:*", "read"))
	require.NoError(t, err)
	require.True(t, decision.Allowed)

	roleNames, err := adapter.RoleNamesForSubject(ctx, subject, "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []string{"iam:admin"}, roleNames)

	businessPermissions, err := adapter.PermissionsForSubject(ctx, subject, "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []authzDomain.Permission{
		mustPermission(t, "iam:admin", "tenant-a", "iam:user:*", "read"),
	}, businessPermissions)

	rolePermissions, err := adapter.PermissionsForRole(ctx, "iam:admin", "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []authzDomain.Permission{
		mustPermission(t, "iam:admin", "tenant-a", "iam:user:*", "read"),
	}, rolePermissions)

	require.NoError(t, adapter.removePolicyFacts(ctx, PolicyRule{Sub: "role:iam:admin", Dom: "tenant-a", Obj: "iam:user:*", Act: "read"}))
	adapter.InvalidateCache()
	allowed, err = adapter.AuthorizeRoute(ctx, "user:100", "tenant-a", "iam:user:*", "read")
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestCasbinAdapterReloadsPolicyFactsFromDatabase(t *testing.T) {
	t.Parallel()

	db := setupCasbinDB(t)
	adapter := newCasbinAdapterForTest(t, db)
	ctx := context.Background()

	require.NoError(t, db.Exec(
		"INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4) VALUES (?, ?, ?, ?, ?, ?)",
		"p", "role:iam:admin", "tenant-a", "iam:user:*", "read", "all:*",
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES (?, ?, ?, ?)",
		"g", "user:100", "role:iam:admin", "tenant-a",
	).Error)

	require.NoError(t, adapter.LoadPolicy(ctx))

	allowed, err := adapter.AuthorizeRoute(ctx, "user:100", "tenant-a", "iam:user:*", "read")
	require.NoError(t, err)
	require.True(t, allowed)

	healthy, reloadErr, reloadAt := adapter.ReloadHealth()
	require.True(t, healthy)
	require.NoError(t, reloadErr)
	require.False(t, reloadAt.IsZero())
}

func TestCasbinAdapterNormalizesLegacyEmptyPolicyScope(t *testing.T) {
	t.Parallel()

	db := setupCasbinDB(t)
	_, err := gormadapter.NewAdapterByDB(db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4) VALUES (?, ?, ?, ?, ?, ?)",
		"p", "role:origin-admin", "tenant-a", "iam:users", "update", "",
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES (?, ?, ?, ?)",
		"g", "user:100", "role:origin-admin", "tenant-a",
	).Error)

	adapter := newCasbinAdapterForTest(t, db)

	var scope string
	require.NoError(t, db.Raw(
		"SELECT v4 FROM casbin_rule WHERE ptype = ? AND v0 = ?",
		"p", "role:origin-admin",
	).Scan(&scope).Error)
	require.Equal(t, "all:*", scope)

	subject, err := authzDomain.NewSubject(authzDomain.SubjectTypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	originScope, err := authzDomain.NewScope(authzDomain.ScopeKindOrigin, "1")
	require.NoError(t, err)
	decision, err := adapter.Check(context.Background(), mustAuthorizationRequestWithScope(t, subject, "tenant-a", "iam:users", "update", originScope))
	require.NoError(t, err)
	require.True(t, decision.Allowed)
}

func TestCasbinAdapterEnforcesScopedPolicyFacts(t *testing.T) {
	t.Parallel()

	adapter := setupCasbinAdapter(t)
	ctx := context.Background()
	require.NoError(t, adapter.addGroupingFacts(ctx, GroupingRule{Sub: "user:100", Dom: "tenant-a", Role: "role:origin-admin"}))
	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:origin-admin", Dom: "tenant-a", Obj: "iam:users", Act: "update", Scope: "origin:1"}))

	subject, err := authzDomain.NewSubject(authzDomain.SubjectTypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	scope1, err := authzDomain.NewScope(authzDomain.ScopeKindOrigin, "1")
	require.NoError(t, err)
	scope2, err := authzDomain.NewScope(authzDomain.ScopeKindOrigin, "2")
	require.NoError(t, err)

	allowed, err := adapter.Check(ctx, mustAuthorizationRequestWithScope(t, subject, "tenant-a", "iam:users", "update", scope1))
	require.NoError(t, err)
	require.True(t, allowed.Allowed)

	denied, err := adapter.Check(ctx, mustAuthorizationRequestWithScope(t, subject, "tenant-a", "iam:users", "update", scope2))
	require.NoError(t, err)
	require.False(t, denied.Allowed)

	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:origin-admin", Dom: "tenant-a", Obj: "iam:profiles", Act: "read", Scope: "all:*"}))
	allAllowed, err := adapter.Check(ctx, mustAuthorizationRequestWithScope(t, subject, "tenant-a", "iam:profiles", "read", scope2))
	require.NoError(t, err)
	require.True(t, allAllowed.Allowed)
}

func setupCasbinAdapter(t *testing.T) *CasbinAdapter {
	t.Helper()
	return newCasbinAdapterForTest(t, setupCasbinDB(t))
}

func newCasbinAdapterForTest(t *testing.T, db *gorm.DB) *CasbinAdapter {
	t.Helper()

	adapter, err := NewCasbinAdapter(db, "model.conf")
	require.NoError(t, err)
	return adapter
}

func setupCasbinDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func mustAuthorizationRequest(t *testing.T, subject authzDomain.Subject, tenantID, resourceKey, action string) authzDomain.AuthorizationRequest {
	t.Helper()
	request, err := authzDomain.NewAuthorizationRequest(subject, tenantID, resourceKey, action)
	require.NoError(t, err)
	return request
}

func mustAuthorizationRequestWithScope(t *testing.T, subject authzDomain.Subject, tenantID, resourceKey, action string, scope authzDomain.Scope) authzDomain.AuthorizationRequest {
	t.Helper()
	request, err := authzDomain.NewAuthorizationRequest(subject, tenantID, resourceKey, action, authzDomain.WithObjectScope(scope))
	require.NoError(t, err)
	return request
}

func mustPermission(t *testing.T, roleName, tenantID, resourceKey, action string) authzDomain.Permission {
	t.Helper()
	permission, err := authzDomain.NewPermission(roleName, tenantID, resourceKey, action)
	require.NoError(t, err)
	return permission
}
