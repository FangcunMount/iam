package casbin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
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
	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:iam:admin", Dom: "tenant-a", Obj: "iam:identity:collection:users", Act: "read"}))

	allowed, err := adapter.AuthorizeRoute(ctx, "user:100", "tenant-a", "iam:identity:collection:users", "read")
	require.NoError(t, err)
	require.True(t, allowed)

	roles, err := adapter.implicitRolesForUser(ctx, "user:100", "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []string{"role:iam:admin"}, roles)

	permissions, err := adapter.implicitPermissionsForUser(ctx, "user:100", "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []PolicyRule{
		{Sub: "role:iam:admin", Dom: "tenant-a", Obj: "iam:identity:collection:users", Act: "read", Scope: "all:*"},
	}, permissions)

	sub, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	decision, err := adapter.Check(ctx, mustAuthorizationRequest(t, sub, "tenant-a", "iam:identity:collection:users", "read"))
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, "allowed", string(decision.Reason))
	require.Equal(t, "iam:admin", decision.MatchedRole)
	require.NotNil(t, decision.MatchedPermission)
	require.Equal(t, "iam:identity:collection:users", decision.MatchedPermission.ResourceKeyString())
	require.False(t, decision.EvaluatedAt.IsZero())

	roleNames, err := adapter.RoleNamesForSubject(ctx, sub, "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []string{"iam:admin"}, roleNames)

	businessPermissions, err := adapter.PermissionsForSubject(ctx, sub, "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []permission.Permission{
		mustPermission(t, "iam:admin", "tenant-a", "iam:identity:collection:users", "read"),
	}, businessPermissions)

	rolePermissions, err := adapter.PermissionsForRole(ctx, "iam:admin", "tenant-a")
	require.NoError(t, err)
	require.Equal(t, []permission.Permission{
		mustPermission(t, "iam:admin", "tenant-a", "iam:identity:collection:users", "read"),
	}, rolePermissions)

	require.NoError(t, adapter.removePolicyFacts(ctx, PolicyRule{Sub: "role:iam:admin", Dom: "tenant-a", Obj: "iam:identity:collection:users", Act: "read"}))
	adapter.InvalidateCache()
	allowed, err = adapter.AuthorizeRoute(ctx, "user:100", "tenant-a", "iam:identity:collection:users", "read")
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
		"p", "role:iam:admin", "tenant-a", "iam:identity:collection:users", "read", "all:*",
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES (?, ?, ?, ?)",
		"g", "user:100", "role:iam:admin", "tenant-a",
	).Error)

	require.NoError(t, adapter.LoadPolicy(ctx))

	allowed, err := adapter.AuthorizeRoute(ctx, "user:100", "tenant-a", "iam:identity:collection:users", "read")
	require.NoError(t, err)
	require.True(t, allowed)

	healthy, reloadErr, reloadAt := adapter.ReloadHealth()
	require.True(t, healthy)
	require.NoError(t, reloadErr)
	require.False(t, reloadAt.IsZero())
}

func TestCasbinRuntimeHealthDetailsExposePolicySyncChannel(t *testing.T) {
	t.Parallel()

	adapter := setupCasbinAdapter(t)
	adapter.SetPolicySyncChannel("iam-policy-sync.host.1001#ephemeral")

	details := adapter.RuntimeHealthDetails()

	require.Equal(t, "iam-policy-sync.host.1001#ephemeral", details["policy_sync_channel"])
}

func TestCasbinAdapterNormalizesLegacyEmptyPolicyScope(t *testing.T) {
	t.Parallel()

	db := setupCasbinDB(t)
	_, err := gormadapter.NewAdapterByDB(db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4) VALUES (?, ?, ?, ?, ?, ?)",
		"p", "role:origin-admin", "tenant-a", "iam:identity:collection:users", "update", "",
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES (?, ?, ?, ?)",
		"g", "user:100", "role:origin-admin", "tenant-a",
	).Error)

	adapter := newCasbinAdapterForTest(t, db)

	var storedScope string
	require.NoError(t, db.Raw(
		"SELECT v4 FROM casbin_rule WHERE ptype = ? AND v0 = ?",
		"p", "role:origin-admin",
	).Scan(&storedScope).Error)
	require.Equal(t, "all:*", storedScope)

	sub, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	originScope, err := scope.New(scope.KindOrigin, "1")
	require.NoError(t, err)
	decision, err := adapter.Check(context.Background(), mustAuthorizationRequestWithScope(t, sub, "tenant-a", "iam:identity:collection:users", "update", originScope))
	require.NoError(t, err)
	require.True(t, decision.Allowed)
}

func TestCasbinAdapterEnforcesScopedPolicyFacts(t *testing.T) {
	t.Parallel()

	adapter := setupCasbinAdapter(t)
	ctx := context.Background()
	require.NoError(t, adapter.addGroupingFacts(ctx, GroupingRule{Sub: "user:100", Dom: "tenant-a", Role: "role:origin-admin"}))
	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:origin-admin", Dom: "tenant-a", Obj: "iam:identity:collection:users", Act: "update", Scope: "origin:1"}))

	sub, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	scope1, err := scope.New(scope.KindOrigin, "1")
	require.NoError(t, err)
	scope2, err := scope.New(scope.KindOrigin, "2")
	require.NoError(t, err)

	allowed, err := adapter.Check(ctx, mustAuthorizationRequestWithScope(t, sub, "tenant-a", "iam:identity:collection:users", "update", scope1))
	require.NoError(t, err)
	require.True(t, allowed.Allowed)

	denied, err := adapter.Check(ctx, mustAuthorizationRequestWithScope(t, sub, "tenant-a", "iam:identity:collection:users", "update", scope2))
	require.NoError(t, err)
	require.False(t, denied.Allowed)
	require.Equal(t, "not_matched", string(denied.Reason))
	require.Equal(t, "policy_not_matched", denied.DenyCode)

	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:origin-admin", Dom: "tenant-a", Obj: "iam:identity:collection:profiles", Act: "read", Scope: "all:*"}))
	allAllowed, err := adapter.Check(ctx, mustAuthorizationRequestWithScope(t, sub, "tenant-a", "iam:identity:collection:profiles", "read", scope2))
	require.NoError(t, err)
	require.True(t, allAllowed.Allowed)
}

func TestCasbinAdapterEnforcesFourSegmentWildcardResourcePatterns(t *testing.T) {
	t.Parallel()

	adapter := setupCasbinAdapter(t)
	ctx := context.Background()
	sub, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)

	require.NoError(t, adapter.addGroupingFacts(ctx, GroupingRule{Sub: "user:100", Dom: "tenant-a", Role: "role:super"}))
	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:super", Dom: "tenant-a", Obj: "*:*:*:*", Act: ".*", Scope: "all:*"}))

	allowed, err := adapter.Check(ctx, mustAuthorizationRequest(t, sub, "tenant-a", "iam:identity:collection:users", "read"))
	require.NoError(t, err)
	require.True(t, allowed.Allowed)

	require.NoError(t, adapter.addGroupingFacts(ctx, GroupingRule{Sub: "user:200", Dom: "tenant-a", Role: "role:qs:admin"}))
	qsSubject, err := subject.NewRef(subject.TypeUser, meta.FromUint64(200))
	require.NoError(t, err)
	require.NoError(t, adapter.addPolicyFacts(ctx, PolicyRule{Sub: "role:qs:admin", Dom: "tenant-a", Obj: "qs:*:*:*", Act: "read|list", Scope: "all:*"}))

	qsAllowed, err := adapter.Check(ctx, mustAuthorizationRequest(t, qsSubject, "tenant-a", "qs:actor:collection:testees", "read"))
	require.NoError(t, err)
	require.True(t, qsAllowed.Allowed)

	qsListAllowed, err := adapter.Check(ctx, mustAuthorizationRequest(t, qsSubject, "tenant-a", "qs:actor:collection:testees", "list"))
	require.NoError(t, err)
	require.True(t, qsListAllowed.Allowed)

	iamDenied, err := adapter.Check(ctx, mustAuthorizationRequest(t, qsSubject, "tenant-a", "iam:identity:collection:users", "read"))
	require.NoError(t, err)
	require.False(t, iamDenied.Allowed)
}

func TestCasbinResourceAndActionMatchers(t *testing.T) {
	require.True(t, resourceMatch("iam:identity:collection:users", "*:*:*:*"))
	require.True(t, resourceMatch("qs:actor:collection:testees", "qs:*:*:*"))
	require.False(t, resourceMatch("iam:identity:collection:users", "qs:*:*:*"))
	require.False(t, resourceMatch("legacy:key", "*:*:*:*"))

	require.True(t, actionMatch("read", "read|list"))
	require.True(t, actionMatch("delete", ".*"))
	require.False(t, actionMatch("reader", "read|list"))
	require.False(t, actionMatch("read", "["))
}

func TestCasbinModelConfigContainsFourSegmentMatchers(t *testing.T) {
	data, err := os.ReadFile(casbinModelPathForTest(t))
	require.NoError(t, err)
	model := string(data)
	require.Contains(t, model, "resourceMatch(r.obj, p.obj)")
	require.Contains(t, model, "actionMatch(r.act, p.act)")
	require.NotContains(t, model, "keyMatch(r.obj, p.obj)")
	require.NotContains(t, model, "regexMatch(r.act, p.act)")
}

func setupCasbinAdapter(t *testing.T) *CasbinAdapter {
	t.Helper()
	return newCasbinAdapterForTest(t, setupCasbinDB(t))
}

func newCasbinAdapterForTest(t *testing.T, db *gorm.DB) *CasbinAdapter {
	t.Helper()

	adapter, err := NewCasbinAdapter(db, casbinModelPathForTest(t))
	require.NoError(t, err)
	return adapter
}

func casbinModelPathForTest(t *testing.T) string {
	t.Helper()
	return filepath.Join(casbinRepoRoot(t), "configs", "casbin_model.conf")
}

func casbinRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func setupCasbinDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func mustAuthorizationRequest(t *testing.T, subject subject.Ref, tenantID, resourceKey, action string) decision.Request {
	t.Helper()
	request, err := decision.NewRequest(subject, tenantID, resourceKey, action)
	require.NoError(t, err)
	return request
}

func mustAuthorizationRequestWithScope(t *testing.T, subject subject.Ref, tenantID, resourceKey, action string, objectScope scope.Scope) decision.Request {
	t.Helper()
	request, err := decision.NewRequest(subject, tenantID, resourceKey, action, decision.WithObjectScope(objectScope))
	require.NoError(t, err)
	return request
}

func mustPermission(t *testing.T, roleName, tenantID, resourceKey, action string) permission.Permission {
	t.Helper()
	permission, err := permission.New(roleName, tenantID, resourceKey, action)
	require.NoError(t, err)
	return permission
}
