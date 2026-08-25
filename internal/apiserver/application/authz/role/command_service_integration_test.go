package role_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	authztestutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/testutil"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	permissiongrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestDeleteRoleRejectsRoleWithActiveGrant(t *testing.T) {
	fixture := authztestutil.NewFixture(t, nil)
	roles := fixture.Roles
	grants := fixture.PermissionGrants
	role, err := roleDomain.NewRole("qs:evaluator", "Evaluator", "tenant-a")
	require.NoError(t, err)
	require.NoError(t, roles.Create(context.Background(), &role))
	grant, err := permissiongrantDomain.New(
		role.ID, "tenant-a", resource.NewResourceID(91), "qs:evaluation:collection:assessments", "retry", constraint.Empty(), "operator",
	)
	require.NoError(t, err)
	require.NoError(t, grants.Create(context.Background(), &grant))
	catalog := roleApp.NewRoleCatalog(roleDomain.NewValidator(roles), fixture.UnitOfWork, nil)

	err = catalog.DeleteRole(context.Background(), roleApp.DeleteRoleCommand{ID: role.ID, TenantID: "tenant-a", ChangedBy: "operator"})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrRoleInUse))
	persisted, err := roles.FindByID(context.Background(), role.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
}
