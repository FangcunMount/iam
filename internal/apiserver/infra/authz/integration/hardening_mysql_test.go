package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	grantapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/permissiongrant"
	resourceapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	grantrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	resourcerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	inheritancerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzdb"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedRole(t *testing.T, db *gorm.DB, name string) role.Role {
	t.Helper()
	r, err := role.NewRole(name, name, "a")
	require.NoError(t, err)
	require.NoError(t, rolerepo.NewRoleRepository(db).Create(context.Background(), &r))
	return r
}
func concurrent(t *testing.T, a, b func() error) {
	t.Helper()
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; results <- a() }()
	go func() { <-start; results <- b() }()
	close(start)
	first, second := <-results, <-results
	require.True(t, (first == nil) != (second == nil), "exactly one conflicting operation must succeed: %v, %v", first, second)
}
func TestMySQLInheritanceAtomicGraphValidation(t *testing.T) {
	db := authzdb.Open(t, true)
	repo := inheritancerepo.NewRepository(db)
	ctx := context.Background()
	a, b := seedRole(t, db, "a"), seedRole(t, db, "b")
	ab, err := roleinheritance.New(a.ID, b.ID, "a", "seed")
	require.NoError(t, err)
	ba, err := roleinheritance.New(b.ID, a.ID, "a", "seed")
	require.NoError(t, err)
	concurrent(t, func() error { return repo.CreateChecked(ctx, &ab) }, func() error { return repo.CreateChecked(ctx, &ba) })
	data, err := authzruntime.NewMySQLSource(db).Load(ctx)
	require.NoError(t, err)
	_, err = authzruntime.BuildSnapshot(data, time.Time{})
	require.NoError(t, err)
	roles := make([]role.Role, 33)
	for i := range roles {
		roles[i] = seedRole(t, db, fmt.Sprintf("depth%d", i))
	}
	for i := 0; i < 31; i++ {
		edge, err := roleinheritance.New(roles[i].ID, roles[i+1].ID, "a", "seed")
		require.NoError(t, err)
		require.NoError(t, repo.CreateChecked(ctx, &edge))
	}
	edge, err := roleinheritance.New(roles[31].ID, roles[32].ID, "a", "seed")
	require.NoError(t, err)
	require.Error(t, repo.CreateChecked(ctx, &edge))
}
func TestMySQLRoleDeletionAndGrantSerialize(t *testing.T) {
	db := authzdb.Open(t, true)
	ctx := context.Background()
	r := seedRole(t, db, "reader")
	res, err := resource.NewResource("example:catalog:collection:documents", []string{"read"}, resource.WithDisplayName("Documents"))
	require.NoError(t, err)
	require.NoError(t, resourcerepo.NewResourceRepository(db).Create(ctx, &res))
	uow := authzuow.NewUnitOfWork(db, nil, authzdb.Stager(t, db))
	grants := grantapp.NewService(uow, grantrepo.NewRepository(db), nil)
	roles := roleapp.NewRoleCatalog(uow, nil)
	concurrent(t, func() error {
		_, err := grants.Create(ctx, grantapp.CreateCommand{TenantID: "a", RoleID: r.ID, ResourceID: res.ID, Action: "read", Constraints: constraint.Empty(), GrantedBy: "seed"})
		return err
	}, func() error {
		return roles.DeleteRole(ctx, roleapp.DeleteRoleCommand{ID: r.ID, TenantID: "a", ChangedBy: "seed"})
	})
	data, err := authzruntime.NewMySQLSource(db).Load(ctx)
	require.NoError(t, err)
	_, err = authzruntime.BuildSnapshot(data, time.Time{})
	require.NoError(t, err)
}

type platformAdmission struct{}

func (platformAdmission) RequireCatalogWrite(context.Context, subject.Ref, string) error { return nil }
func TestMySQLResourceUpdateAndGrantSerialize(t *testing.T) {
	db := authzdb.Open(t, true)
	ctx := context.Background()
	r := seedRole(t, db, "reader")
	res, err := resource.NewResource("example:catalog:collection:documents", []string{"read", "use"}, resource.WithDisplayName("Documents"))
	require.NoError(t, err)
	require.NoError(t, resourcerepo.NewResourceRepository(db).Create(ctx, &res))
	uow := authzuow.NewUnitOfWork(db, nil, authzdb.Stager(t, db))
	actor, err := subject.NewUserRef(meta.ID(1))
	require.NoError(t, err)
	grants := grantapp.NewService(uow, grantrepo.NewRepository(db), nil)
	resources := resourceapp.NewResourceCatalog(uow, nil, platformAdmission{})
	concurrent(t, func() error {
		_, err := grants.Create(ctx, grantapp.CreateCommand{TenantID: "a", RoleID: r.ID, ResourceID: res.ID, Action: "use", Constraints: constraint.Empty(), GrantedBy: "seed"})
		return err
	}, func() error {
		_, err := resources.UpdateResource(ctx, resourceapp.UpdateResourceCommand{ID: res.ID, TenantID: "platform", ChangedBy: "seed", Actor: actor, Actions: []string{"read"}})
		return err
	})
	data, err := authzruntime.NewMySQLSource(db).Load(ctx)
	require.NoError(t, err)
	_, err = authzruntime.BuildSnapshot(data, time.Time{})
	require.NoError(t, err)
}
