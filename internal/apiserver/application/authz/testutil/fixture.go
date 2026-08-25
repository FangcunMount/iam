package testutil

import (
	"fmt"
	"os"
	"testing"

	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	permissionGrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	roleInheritanceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	permissionGrantRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourceRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	roleRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	roleBindingRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/rolebinding"
	roleInheritanceRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	mysqlAuthzUOW "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Fixture struct {
	Roles            roleDomain.Repository
	Resources        resourceDomain.Repository
	PermissionGrants permissionGrantDomain.Repository
	RoleInheritances roleInheritanceDomain.Repository
	UnitOfWork       authzuow.UnitOfWork
	db               *gorm.DB
}

func NewFixture(t *testing.T, stager event.Stager) *Fixture {
	t.Helper()
	temporary, err := os.CreateTemp("", "iam-authz-test-*.db")
	require.NoError(t, err)
	path := temporary.Name()
	require.NoError(t, temporary.Close())
	t.Cleanup(func() { _ = os.Remove(path) })

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=10000", path)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	require.NoError(t, db.AutoMigrate(
		&roleRepo.RolePO{}, &roleBindingRepo.BindingPO{}, &resourceRepo.ResourcePO{},
		&permissionGrantRepo.GrantPO{}, &roleInheritanceRepo.InheritancePO{}, &policyRepo.PolicyVersionPO{},
	))
	return &Fixture{
		Roles:            roleRepo.NewRoleRepository(db),
		Resources:        resourceRepo.NewResourceRepository(db),
		PermissionGrants: permissionGrantRepo.NewRepository(db),
		RoleInheritances: roleInheritanceRepo.NewRepository(db),
		UnitOfWork:       mysqlAuthzUOW.NewUnitOfWork(db, nil, stager),
		db:               db,
	}
}

func (f *Fixture) PolicyVersionCount(t *testing.T) int64 {
	t.Helper()
	return f.count(t, &policyRepo.PolicyVersionPO{})
}

func (f *Fixture) RoleInheritanceCount(t *testing.T) int64 {
	t.Helper()
	return f.count(t, &roleInheritanceRepo.InheritancePO{})
}

func (f *Fixture) count(t *testing.T, model any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, f.db.Model(model).Count(&count).Error)
	return count
}
