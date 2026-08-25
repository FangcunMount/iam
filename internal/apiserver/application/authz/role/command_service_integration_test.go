package role_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	permissiongrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	permissiongrantRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	roleRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	rolebindingRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/rolebinding"
	roleinheritanceRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	authzUOW "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteRoleRejectsRoleWithActiveGrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&roleRepo.RolePO{}, &rolebindingRepo.BindingPO{}, &permissiongrantRepo.GrantPO{}, &roleinheritanceRepo.InheritancePO{},
	))
	roles := roleRepo.NewRoleRepository(db)
	grants := permissiongrantRepo.NewRepository(db)
	role, err := roleDomain.NewRole("qs:evaluator", "Evaluator", "tenant-a")
	require.NoError(t, err)
	require.NoError(t, roles.Create(context.Background(), &role))
	grant, err := permissiongrantDomain.New(
		role.ID, "tenant-a", resource.NewResourceID(91), "qs:evaluation:collection:assessments", "retry", constraint.Empty(), "operator",
	)
	require.NoError(t, err)
	require.NoError(t, grants.Create(context.Background(), &grant))
	catalog := roleApp.NewRoleCatalog(roleDomain.NewValidator(roles), authzUOW.NewUnitOfWork(db, nil), nil)

	err = catalog.DeleteRole(context.Background(), roleApp.DeleteRoleCommand{ID: role.ID, TenantID: "tenant-a", ChangedBy: "operator"})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrRoleInUse))
	persisted, err := roles.FindByID(context.Background(), role.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
}
