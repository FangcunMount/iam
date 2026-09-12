package migration

import (
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"testing"
)

func TestAssignmentScopeUpgradeAndRollbackMySQL(t *testing.T) {
	if os.Getenv("IAM_SCOPE_MYSQL_REQUIRED") == "1" {
		require.NotEmpty(t, os.Getenv("MYSQL_HOST"), "Scope MySQL test requires isolated MySQL")
	}
	db := openMigrationMySQL(t)
	database := migrationEnvOr("MYSQL_DATABASE", "iam_test")
	var tables int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=?", database).Scan(&tables))
	require.Zero(t, tables, "requires isolated empty database")
	preparation, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: openMigrationMySQL(t)}), &gorm.Config{})
	require.NoError(t, err)
	_, _, err = NewMigrator(db, &Config{Enabled: true, Database: database, FreshStages: freshStagesForTest(t, preparation)}).Run()
	require.NoError(t, err)
	pool := openMigrationMySQL(t)

	up, err := migrations.ReadFile("migrations/000036_assignment_data_scope.up.sql")
	require.NoError(t, err)
	down, err := migrations.ReadFile("migrations/000036_assignment_data_scope.down.sql")
	require.NoError(t, err)

	_, err = pool.Exec(string(down))
	require.NoError(t, err)
	_, err = pool.Exec("INSERT INTO authz_assignments(id,subject_type,subject_id,role_id,granted_by,granted_at) VALUES(990036001,'user','990036001',1,'test',NOW())")
	require.NoError(t, err)
	_, err = pool.Exec(string(up))
	require.NoError(t, err)
	var org uint64
	var kind string
	var missing bool
	require.NoError(t, pool.QueryRow("SELECT org_id,scope_kind,scope_store_ids IS NULL FROM authz_assignments WHERE id=990036001").Scan(&org, &kind, &missing))
	require.Zero(t, org)
	require.Empty(t, kind)
	require.True(t, missing)
	// An unused scope schema can be removed without rewriting historical facts.
	_, err = pool.Exec(string(down))
	require.NoError(t, err)
	_, err = pool.Exec(string(up))
	require.NoError(t, err)
	for _, company := range []uint64{1, 2} {
		_, err = pool.Exec("INSERT INTO authz_assignments(id,subject_type,subject_id,role_id,granted_by,granted_at,org_id,scope_kind,scope_store_ids) VALUES(?,'user','990036001',1,'test',NOW(),?,'stores',JSON_ARRAY('123456789012345678'))", 990036001+company, company)
		require.NoError(t, err)
	}
	_, err = pool.Exec("INSERT INTO authz_assignments(id,subject_type,subject_id,role_id,granted_by,granted_at,org_id,scope_kind,scope_store_ids) VALUES(990036009,'user','990036001',1,'test',NOW(),1,'all_stores',JSON_ARRAY())")
	require.Error(t, err, "duplicate within company must be rejected")
	_, err = pool.Exec(string(down))
	require.Error(t, err, "configured scopes block structural rollback")
	var scoped int
	require.NoError(t, pool.QueryRow("SELECT COUNT(*) FROM authz_assignments WHERE org_id IN (1,2) AND subject_id='990036001'").Scan(&scoped))
	require.Equal(t, 2, scoped)
}
