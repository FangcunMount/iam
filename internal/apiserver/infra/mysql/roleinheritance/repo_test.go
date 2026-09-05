package roleinheritance_test

import (
	"context"
	rolepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	repo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryRejectsCycleAndAllowsRegrantAfterRevoke(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repo.InheritancePO{}, &rolepo.RolePO{}))
	for i := uint64(1); i <= 3; i++ {
		require.NoError(t, db.Exec("INSERT INTO authz_roles (id,name,display_name,tenant_id,version) VALUES (?,?,?,?,1)", i, meta.FromUint64(i).String(), "Role", "tenant-a").Error)
	}
	repository := repo.NewRepository(db)
	ctx := context.Background()

	edgeAB := mustInheritance(t, 1, 2)
	require.NoError(t, repository.CreateChecked(ctx, &edgeAB))
	edgeBC := mustInheritance(t, 2, 3)
	require.NoError(t, repository.CreateChecked(ctx, &edgeBC))
	edgeCA := mustInheritance(t, 3, 1)
	err = repository.CreateChecked(ctx, &edgeCA)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	outcome, err := repository.AtomicRevoke(ctx, edgeAB.ID, "tenant-a")
	require.NoError(t, err)
	require.Equal(t, domain.RevokeOutcomeRevoked, outcome)
	regrant := mustInheritance(t, 1, 2)
	require.NoError(t, repository.CreateChecked(ctx, &regrant))
}

func mustInheritance(t *testing.T, roleID, inheritedRoleID uint64) domain.Inheritance {
	t.Helper()
	inheritance, err := domain.New(meta.FromUint64(roleID), meta.FromUint64(inheritedRoleID), "tenant-a", "operator-1")
	require.NoError(t, err)
	return inheritance
}
