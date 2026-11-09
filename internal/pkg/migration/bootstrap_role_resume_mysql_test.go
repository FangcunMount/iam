package migration

import (
	"errors"
	"os"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestFreshRoleBootstrapResumesAfterCommittedApplyMySQL(t *testing.T) {
	if os.Getenv("IAM_M5_BOOTSTRAP_ATOMIC_FIXTURE") != "1" {
		t.Skip("dedicated disposable MySQL fixture required")
	}
	require.Equal(t, "127.0.0.1", os.Getenv("MYSQL_HOST"))
	require.Equal(t, "iam_test", os.Getenv("MYSQL_DATABASE"))
	ctx := t.Context()
	read := openMigrationMySQL(t)
	count := func(query string) int {
		t.Helper()
		var value int
		require.NoError(t, read.QueryRowContext(ctx, query).Scan(&value))
		return value
	}
	require.Zero(t, count("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()"))
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: openMigrationMySQL(t)}), &gorm.Config{})
	require.NoError(t, err)
	advance := func(target uint) {
		t.Helper()
		migrator := NewMigrator(openMigrationMySQL(t), &Config{Enabled: true, Database: "iam_test"})
		version, changed, err := migrator.RunTo(target)
		require.NoError(t, err)
		require.Equal(t, target, version)
		require.True(t, changed)
	}
	stages := freshStagesForTest(t, db)
	advance(31)
	require.NoError(t, stages[0].Prepare())
	advance(33)
	rejected := errors.New("fixture interruption before inheritance archive insert")
	const callback = "fixture:reject_inheritance_archive"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == (rolemodel.InheritanceArchive{}).TableName() {
			_ = tx.AddError(rejected)
		}
	}))
	require.ErrorIs(t, stages[1].Prepare(), rejected)
	require.NoError(t, db.Callback().Create().Remove(callback))
	const pending = "SELECT COUNT(*) FROM domain_event_outbox WHERE status='pending'"
	const archives = "SELECT COUNT(*) FROM iam_role_inheritance_archives"
	const policy = "SELECT policy_version FROM authz_policy_versions WHERE id=1"
	require.Equal(t, 1, count(pending))
	require.Zero(t, count(archives))
	require.Equal(t, 1, count("SELECT COUNT(*) FROM iam_role_model_migrations WHERE status='applied'"))
	version := count(policy)
	var notice string
	require.NoError(t, read.QueryRowContext(ctx, "SELECT CONCAT(event_id,':',SHA2(payload_json,256)) FROM domain_event_outbox WHERE status='pending'").Scan(&notice))

	_, err = read.ExecContext(ctx, "UPDATE authz_policy_versions SET policy_version=policy_version+1 WHERE id=1")
	require.NoError(t, err)
	require.Error(t, rolemodel.BootstrapIndependentRoles(ctx, db, nil))
	require.Equal(t, version+1, count(policy))
	require.Equal(t, 1, count(pending))
	require.Zero(t, count(archives))
	_, err = read.ExecContext(ctx, "UPDATE authz_policy_versions SET policy_version=? WHERE id=1", version)
	require.NoError(t, err)

	require.NoError(t, rolemodel.BootstrapIndependentRoles(ctx, db, nil))
	require.NoError(t, rolemodel.BootstrapIndependentRoles(ctx, db, nil))
	require.Equal(t, version, count(policy))
	require.Equal(t, 1, count(pending))
	require.Equal(t, 1, count(archives))
	var after string
	require.NoError(t, read.QueryRowContext(ctx, "SELECT CONCAT(event_id,':',SHA2(payload_json,256)) FROM domain_event_outbox WHERE status='pending'").Scan(&after))
	require.Equal(t, notice, after)
	advance(34)
	require.False(t, db.Migrator().HasTable("authz_role_inheritances"))
}
