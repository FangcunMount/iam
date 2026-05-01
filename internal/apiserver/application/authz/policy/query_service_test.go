package policy

import (
	"context"
	"testing"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
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
		permissions: []authzDomain.Permission{
			mustPolicyPermission(t, "iam:admin", "tenant-a", "iam:user:*", "read"),
		},
	}
	service := NewPolicyQueryService(&policyVersionRepoForCommandStub{}, store, roleRepo)

	permissions, err := service.GetPermissionsForRole(context.Background(), RolePermissionsQuery{
		RoleID:   10,
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
	permissions []authzDomain.Permission
	calls       []struct {
		roleName string
		tenantID string
	}
}

func (s *rolePermissionStoreStub) PermissionsForRole(_ context.Context, roleName, tenantID string) ([]authzDomain.Permission, error) {
	s.calls = append(s.calls, struct {
		roleName string
		tenantID string
	}{roleName: roleName, tenantID: tenantID})
	return s.permissions, nil
}

func mustPolicyPermission(t *testing.T, roleName, tenantID, resourceKey, action string) authzDomain.Permission {
	t.Helper()
	permission, err := authzDomain.NewPermission(roleName, tenantID, resourceKey, action)
	require.NoError(t, err)
	return permission
}

var _ policyDomain.Repository = (*policyVersionRepoForCommandStub)(nil)
