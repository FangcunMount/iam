package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/conditionretire"

	"time"

	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestFreshDatabaseMigratesToIndependentRolesMySQL(t *testing.T) {
	pool := openMigrationMySQL(t)
	database := migrationEnvOr("MYSQL_DATABASE", "iam_test")
	var count int
	require.NoError(t, pool.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=?", database).Scan(&count))
	require.Zero(t, count, "dedicated empty database required")
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: openMigrationMySQL(t)}), &gorm.Config{})
	require.NoError(t, err)
	ctx := context.Background()
	stages := freshStagesForTest(t, db)
	version, changed, err := NewMigrator(pool, &Config{Enabled: true, Database: database, FreshStages: stages}).Run()
	require.NoError(t, err)
	require.True(t, changed)
	require.EqualValues(t, 35, version)
	require.False(t, db.Migrator().HasTable("authz_role_inheritances"))
	_, err = conditionretire.Verify(ctx, db)
	require.NoError(t, err)
	dataset, err := authzruntime.NewMySQLSource(db).Load(ctx)
	require.NoError(t, err)
	_, err = authzruntime.BuildSnapshot(dataset, time.Now())
	require.NoError(t, err)
	require.Len(t, dataset.Roles, 8) // seven managed roles and retained user self-service
}

func freshStagesForTest(t *testing.T, db *gorm.DB) []FreshStage {
	t.Helper()
	catalogPath := os.Getenv("RM_IAM_EVENTS_CATALOG")
	if catalogPath == "" {
		catalogPath = filepath.Join("..", "..", "..", "configs", "events.yaml")
	}
	catalog, err := eventcatalog.Load(catalogPath)
	require.NoError(t, err)
	ctx := context.Background()
	return []FreshStage{
		{Version: 31, Prepare: func() error { return rolemodel.PrepareBootstrapTenant(ctx, db) }},
		{Version: 33, Prepare: func() error {
			return rolemodel.BootstrapIndependentRoles(ctx, db, eventoutbox.NewStore(db, eventcatalog.NewCatalog(catalog)))
		}},
		{Version: 35, Prepare: func() error {
			return conditionretire.Bootstrap(ctx, db, eventoutbox.NewStore(db, eventcatalog.NewCatalog(catalog)))
		}},
	}
}
