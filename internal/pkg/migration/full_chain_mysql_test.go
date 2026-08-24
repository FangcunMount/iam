package migration

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestFullMigrationChainAndBootstrapMySQL(t *testing.T) {
	migrationDB := openMigrationMySQL(t)
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))

	var existingTables int
	if err := migrationDB.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, database).Scan(&existingTables); err != nil {
		t.Fatalf("count existing migration test tables: %v", err)
	}
	if existingTables != 0 {
		t.Fatalf("full-chain migration test requires an empty dedicated database, found %d tables", existingTables)
	}

	version, migrated, err := NewMigrator(migrationDB, &Config{Enabled: true, Database: database}).Run()
	if err != nil {
		t.Fatalf("run full migration chain: %v", err)
	}
	if !migrated || version != 25 {
		t.Fatalf("full migration result = version %d migrated=%v, want version 25 migrated=true", version, migrated)
	}
	db := openMigrationMySQL(t)
	for _, retired := range []string{
		"children", "guardianships", "schema_version", "tenants", "data_dictionary",
		"operation_logs", "audit_logs", "auth_token_audit",
		"auth_accounts", "auth_credentials_legacy",
		"cbpt_profiles_s812v2", "cbpt_profile_links_s812v2",
		"cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1",
		"cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1",
	} {
		assertTableExists(t, db, retired, false)
	}
	assertTableExists(t, db, "schema_migrations", true)
	assertCurrentSchemaTables(t, db, database)

	_, currentFile, _, _ := runtime.Caller(0)
	bootstrapPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "configs", "mysql", "bootstrap.sql")
	bootstrap, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("read bootstrap SQL: %v", err)
	}
	for run := 1; run <= 2; run++ {
		if _, err := db.Exec(string(bootstrap)); err != nil {
			t.Fatalf("bootstrap run %d after current migration: %v", run, err)
		}
	}
	assertCurrentSchemaTables(t, db, database)
	for _, retired := range []string{"tenants", "data_dictionary"} {
		assertTableExists(t, db, retired, false)
	}
	assertMigratedRoleBindingGuardUnderConcurrency(t, db)
}

func assertMigratedRoleBindingGuardUnderConcurrency(t *testing.T, db *sql.DB) {
	t.Helper()
	const concurrency = 20
	const baseID uint64 = 9_250_000_000_000_000_000

	var wg sync.WaitGroup
	errs := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			_, err := db.Exec(`
INSERT INTO authz_assignments
    (id, subject_type, subject_id, role_id, tenant_id, granted_by, granted_at)
VALUES (?, 'user', 'migration-concurrency-user', 424242, 'migration-concurrency-tenant', 'migration-test', UTC_TIMESTAMP())`,
				baseID+uint64(offset),
			)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	successes := 0
	duplicates := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			duplicates++
			continue
		}
		t.Fatalf("concurrent role binding insert failed unexpectedly: %v", err)
	}
	if successes != 1 || duplicates != concurrency-1 {
		t.Fatalf("migrated active RoleBinding guard successes/duplicates = %d/%d, want 1/%d", successes, duplicates, concurrency-1)
	}

	if _, err := db.Exec(`
UPDATE authz_assignments
SET deleted_at = UTC_TIMESTAMP()
WHERE subject_type = 'user'
  AND subject_id = 'migration-concurrency-user'
  AND role_id = 424242
  AND tenant_id = 'migration-concurrency-tenant'
  AND deleted_at IS NULL`); err != nil {
		t.Fatalf("mark migrated role binding historical: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO authz_assignments
    (id, subject_type, subject_id, role_id, tenant_id, granted_by, granted_at)
VALUES (?, 'user', 'migration-concurrency-user', 424242, 'migration-concurrency-tenant', 'migration-test', UTC_TIMESTAMP())`,
		baseID+concurrency+1,
	); err != nil {
		t.Fatalf("re-grant after historical role binding: %v", err)
	}
}

func assertCurrentSchemaTables(t *testing.T, db *sql.DB, database string) {
	t.Helper()
	rows, err := db.Query(`
SELECT TABLE_NAME
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME`, database)
	if err != nil {
		t.Fatalf("query current schema tables: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var got []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scan current schema table: %v", err)
		}
		got = append(got, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate current schema tables: %v", err)
	}

	want := []string{
		"auth_credentials",
		"auth_login_identities",
		"authz_assignments",
		"authz_policy_versions",
		"authz_resources",
		"authz_roles",
		"casbin_rule",
		"domain_event_outbox",
		"identity_session_revocation_outbox",
		"idp_wechat_apps",
		"jwks_keys",
		"profile_links",
		"profiles",
		"schema_migrations",
		"users",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("current schema tables = %v, want %v", got, want)
	}
}
