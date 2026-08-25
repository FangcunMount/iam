package permissiongrant_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	repo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryCreatesRevokesAndAllowsRegrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repo.GrantPO{}))
	repository := repo.NewRepository(db)
	ctx := context.Background()

	first := mustGrant(t)
	require.NoError(t, repository.Create(ctx, &first))
	duplicate := mustGrant(t)
	err = repository.Create(ctx, &duplicate)
	require.True(t, perrors.IsCode(err, code.ErrPermissionGrantAlreadyExists))

	require.NoError(t, repository.Revoke(ctx, first.ID))
	second := mustGrant(t)
	require.NoError(t, repository.Create(ctx, &second))

	active, err := repository.ListActiveByTenant(ctx, "tenant-a")
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, second.ID, active[0].ID)
}

func mustGrant(t *testing.T) domain.Grant {
	t.Helper()
	grant, err := domain.New(
		meta.FromUint64(10), "tenant-a", resource.NewResourceID(20),
		"qs:evaluation:collection:assessments", "retry", constraint.Empty(), "operator-1",
	)
	require.NoError(t, err)
	return grant
}
