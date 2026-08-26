package permissiongrant_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	repo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryCreatesRevokesAndAllowsRegrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	testRepositoryCreatesRevokesAndAllowsRegrant(t, db)
}

func TestRepositoryHistoricalRevokedGrantMySQLConcurrencyRegression(t *testing.T) {
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		t.Skip("MYSQL_HOST is not configured")
	}
	port := os.Getenv("MYSQL_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("MYSQL_USER")
	if user == "" {
		user = os.Getenv("MYSQL_USERNAME")
	}
	database := os.Getenv("MYSQL_DATABASE")
	if database == "" {
		database = os.Getenv("MYSQL_DBNAME")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		user, os.Getenv("MYSQL_PASSWORD"), host, port, database)
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repo.GrantPO{}))
	// The production migration makes audit columns NOT NULL. Keep this test from
	// accepting a nullable AutoMigrate schema that masks system-context revokes.
	require.NoError(t, db.Exec(`ALTER TABLE authz_permission_grants
		MODIFY updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0`).Error)
	require.NoError(t, db.Unscoped().Where("tenant_id = ?", "tenant-a").Delete(&repo.GrantPO{}).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Unscoped().Where("tenant_id = ?", "tenant-a").Delete(&repo.GrantPO{}).Error)
	})
	testRepositoryCreatesRevokesAndAllowsRegrant(t, db)

	result := db.Unscoped().Model(&repo.GrantPO{}).
		Where("revoked_at IS NOT NULL").
		Update("grant_key", strings.Repeat("0", 64))
	require.NoError(t, result.Error)
	require.EqualValues(t, 1, result.RowsAffected)

	repository := repo.NewRepository(db)
	_, err = repository.ListByRole(context.Background(), meta.FromUint64(10), "tenant-a")
	require.Error(t, err)
	active, err := repository.ListActiveByTenant(context.Background(), "tenant-a")
	require.NoError(t, err)
	require.Len(t, active, 1)
}

func testRepositoryCreatesRevokesAndAllowsRegrant(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&repo.GrantPO{}))
	repository := repo.NewRepository(db)
	ctx := context.Background()

	first := mustGrant(t)
	require.NoError(t, repository.Create(ctx, &first))
	duplicate := mustGrant(t)
	err := repository.Create(ctx, &duplicate)
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
