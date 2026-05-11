package policy

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPolicyQueryServiceReturnsBusinessPermissionsForRole(t *testing.T) {
	t.Parallel()

	roleRepo := &permissionRoleRepoStub{
		role: &roleDomain.Role{
			ID:       meta.FromUint64(10),
			Name:     "iam:admin",
			TenantID: "tenant-a",
		},
	}
	store := &rolePermissionStoreStub{
		permissions: []permission.Permission{
			mustPolicyPermission(t, "iam:admin", "tenant-a", "iam:identity:collection:users", "read"),
		},
	}
	service := NewPolicyQueryService(&policyVersionRepoForCommandStub{}, store, roleRepo)

	permissions, err := service.GetPermissionsForRole(context.Background(), RolePermissionsQuery{
		RoleID:   meta.FromUint64(10),
		TenantID: "tenant-a",
	})

	require.NoError(t, err)
	require.Equal(t, store.permissions, permissions)
	require.Equal(t, []struct {
		roleName string
		tenantID string
	}{{roleName: "iam:admin", tenantID: "tenant-a"}}, store.calls)
}

type permissionRoleRepoStub struct {
	role *roleDomain.Role
}

func (r *permissionRoleRepoStub) Create(context.Context, *roleDomain.Role) error { return nil }
func (r *permissionRoleRepoStub) Update(context.Context, *roleDomain.Role) error { return nil }
func (r *permissionRoleRepoStub) Delete(context.Context, meta.ID) error          { return nil }
func (r *permissionRoleRepoStub) FindByID(context.Context, meta.ID) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *permissionRoleRepoStub) FindByName(context.Context, string, string) (*roleDomain.Role, error) {
	return r.role, nil
}
func (r *permissionRoleRepoStub) List(context.Context, string, int, int) ([]*roleDomain.Role, int64, error) {
	return nil, 0, nil
}

type rolePermissionStoreStub struct {
	permissions []permission.Permission
	calls       []struct {
		roleName string
		tenantID string
	}
}

func (s *rolePermissionStoreStub) PermissionsForRole(_ context.Context, roleName, tenantID string) ([]permission.Permission, error) {
	s.calls = append(s.calls, struct {
		roleName string
		tenantID string
	}{roleName: roleName, tenantID: tenantID})
	return s.permissions, nil
}

func mustPolicyPermission(t *testing.T, roleName, tenantID, resourceKey, action string) permission.Permission {
	t.Helper()
	permission, err := permission.New(roleName, tenantID, resourceKey, action)
	require.NoError(t, err)
	return permission
}

var _ policyDomain.Repository = (*policyVersionRepoForCommandStub)(nil)
