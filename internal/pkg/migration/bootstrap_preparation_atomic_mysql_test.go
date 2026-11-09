package migration

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestFreshTenantPreparationRollsBackBeforeReceiptMySQL(t *testing.T) {
	if os.Getenv("IAM_M5_BOOTSTRAP_ATOMIC_FIXTURE") != "1" {
		t.Skip("dedicated disposable MySQL fixture required")
	}
	require.Equal(t, "127.0.0.1", os.Getenv("MYSQL_HOST"))
	require.Equal(t, "iam_test", os.Getenv("MYSQL_DATABASE"))
	ctx := t.Context()
	pool := openMigrationMySQL(t)
	var count int
	require.NoError(t, pool.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()").Scan(&count))
	require.Zero(t, count, "requires an empty isolated database")
	migrator := NewMigrator(pool, &Config{Enabled: true, Database: "iam_test"})
	version, changed, err := migrator.RunTo(31)
	require.NoError(t, err)
	require.EqualValues(t, 31, version)
	require.True(t, changed)

	read := openMigrationMySQL(t)
	state := func() string {
		t.Helper()
		var roles string
		require.NoError(t, read.QueryRowContext(ctx, `SELECT GROUP_CONCAT(
			CONCAT_WS('|',id,name,IFNULL(deleted_at,'')) ORDER BY id)
			FROM authz_roles`).Scan(&roles))
		require.NotEmpty(t, roles)
		return roles
	}
	before := state()
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: openMigrationMySQL(t)}), &gorm.Config{})
	require.NoError(t, err)
	rejected := errors.New("fixture rejects final preparation receipt")
	const callback = "fixture:reject_preparation_receipt"
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(callback, func(tx *gorm.DB) {
		if strings.HasPrefix(tx.Statement.SQL.String(), "INSERT INTO iam_authorization_retirement_preparation(") {
			_ = tx.AddError(rejected)
		}
	}))
	require.ErrorIs(t, rolemodel.PrepareBootstrapTenant(ctx, db), rejected)
	require.Equal(t, before, state(), "role changes escaped the aborted preparation transaction")
	const receiptCount = "SELECT COUNT(*) FROM iam_authorization_retirement_preparation"
	require.NoError(t, read.QueryRowContext(ctx, receiptCount).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.Callback().Raw().Remove(callback))
	require.NoError(t, rolemodel.PrepareBootstrapTenant(ctx, db))
	require.NoError(t, read.QueryRowContext(ctx, receiptCount).Scan(&count))
	require.Equal(t, 1, count)
	require.NotEqual(t, before, state())
	migrator = NewMigrator(openMigrationMySQL(t), &Config{Enabled: true, Database: "iam_test"})
	version, changed, err = migrator.RunTo(32)
	require.NoError(t, err)
	require.EqualValues(t, 32, version)
	require.True(t, changed)
	require.False(t, db.Migrator().HasTable("iam_authorization_retirement_preparation"))
}
