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

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

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

	preparationDB, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: openMigrationMySQL(t)}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	version, migrated, err := NewMigrator(migrationDB, &Config{Enabled: true, Database: database, FreshStages: freshStagesForTest(t, preparationDB)}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if !migrated || version != 37 {
		t.Fatalf("migration version=%d changed=%v", version, migrated)
	}
	db := openMigrationMySQL(t)
	assertJWKSGraceActionRetired(t, db)
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
	var nonDefaultUsernames int
	if err := db.QueryRow("SELECT COUNT(*) FROM auth_login_identities WHERE provider='username' AND realm <> 'default'").Scan(&nonDefaultUsernames); err != nil || nonDefaultUsernames != 0 {
		t.Fatalf("bootstrap must use default username realm: count=%d err=%v", nonDefaultUsernames, err)
	}
	assertNativeAuthzBootstrap(t, db)
	assertJWKSGraceActionRetired(t, db)
	for _, retired := range []string{"tenants", "data_dictionary"} {
		assertTableExists(t, db, retired, false)
	}
	assertMigratedRoleBindingGuardUnderConcurrency(t, db)
}

func assertJWKSGraceActionRetired(t *testing.T, db *sql.DB) {
	t.Helper()
	var resourceCount, validJSONCount, enterGraceCount int
	if err := db.QueryRow(`
SELECT COUNT(*),
       COALESCE(SUM(JSON_VALID(actions)), 0),
       COALESCE(SUM(JSON_SEARCH(actions, 'one', 'enter_grace') IS NOT NULL), 0)
FROM authz_resources
WHERE `+"`key`"+` = 'iam:authn:collection:jwks'`).Scan(
		&resourceCount,
		&validJSONCount,
		&enterGraceCount,
	); err != nil {
		t.Fatalf("query JWKS resource actions: %v", err)
	}
	if resourceCount != 1 || validJSONCount != 1 || enterGraceCount != 0 {
		t.Fatalf(
			"JWKS resource/action state = resources %d valid JSON %d enter_grace %d, want 1/1/0",
			resourceCount,
			validJSONCount,
			enterGraceCount,
		)
	}
}

func assertNativeAuthzBootstrap(t *testing.T, db *sql.DB) {
	t.Helper()
	for label, query := range map[string]string{
		"active roles":     "SELECT COUNT(*) FROM authz_roles WHERE deleted_at IS NULL",
		"active resources": "SELECT COUNT(*) FROM authz_resources WHERE deleted_at IS NULL",
		"active grants":    "SELECT COUNT(*) FROM authz_permission_grants WHERE revoked_at IS NULL AND deleted_at IS NULL",
	} {
		want := map[string]int{"active roles": 8, "active resources": 26, "active grants": 104}[label]
		var got int
		if err := db.QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("query %s: %v", label, err)
		}
		if got != want {
			t.Fatalf("%s = %d, want %d", label, got, want)
		}
	}
	assertGrantCount := func(name, query string, want int) {
		t.Helper()
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			t.Fatalf("query %s grant: %v", name, err)
		}
		if count != want {
			t.Fatalf("%s grant count = %d, want %d", name, count, want)
		}
	}
	assertGrantCount("tenant catalog read capabilities", `
SELECT COUNT(*)
FROM authz_permission_grants g
JOIN authz_roles r ON r.id = g.role_id AND r.deleted_at IS NULL
WHERE r.name = 'iam_admin'
  AND g.resource_pattern = 'iam:authz:collection:resources'
  AND g.action IN ('read', 'list', 'validate_action')
  AND g.revoked_at IS NULL AND g.deleted_at IS NULL`, 3)
	assertGrantCount("non-platform catalog write grants", `
SELECT COUNT(*)
FROM authz_permission_grants g
JOIN authz_roles r ON r.id = g.role_id AND r.deleted_at IS NULL
WHERE r.management_protection = 'standard'
  AND g.resource_pattern = 'iam:authz:collection:resources'
  AND g.action IN ('create', 'update', 'delete', '*')
  AND g.revoked_at IS NULL AND g.deleted_at IS NULL`, 0)
	assertGrantCount("operator adhoc retry", `
SELECT COUNT(*)
FROM authz_permission_grants g
JOIN authz_roles r ON r.id = g.role_id AND r.deleted_at IS NULL
WHERE r.name = 'qs:assessment_operator'
  AND g.resource_pattern = 'qs:evaluation:collection:assessments'
  AND g.action = 'retry'
  AND JSON_LENGTH(g.constraint_set, '$.all_of') = 0
  AND g.revoked_at IS NULL AND g.deleted_at IS NULL`, 1)
	assertGrantCount("plan manager plan retry", `
SELECT COUNT(*)
FROM authz_permission_grants g
JOIN authz_roles r ON r.id = g.role_id AND r.deleted_at IS NULL
WHERE r.name = 'qs:evaluation_plan_manager'
  AND g.resource_pattern = 'qs:evaluation:collection:assessments'
  AND g.action = 'retry'
  AND JSON_LENGTH(g.constraint_set, '$.all_of') = 0
  AND g.revoked_at IS NULL AND g.deleted_at IS NULL`, 1)
	assertGrantCount("admin unconditional wildcard", `
SELECT COUNT(*)
FROM authz_permission_grants g
JOIN authz_roles r ON r.id = g.role_id AND r.deleted_at IS NULL
WHERE r.name = 'qs:admin'
  AND g.resource_pattern = 'qs:*:*:*'
  AND g.action = '*'
  AND JSON_LENGTH(g.constraint_set, '$.all_of') = 0
  AND g.revoked_at IS NULL AND g.deleted_at IS NULL`, 1)
	assertGrantCount("conditional bulk prohibition", `
SELECT COUNT(*)
FROM authz_permission_grants
WHERE JSON_LENGTH(constraint_set, '$.all_of') > 0
  AND (action IN ('list', 'search', 'batch') OR action LIKE 'batch\\_%')
  AND revoked_at IS NULL AND deleted_at IS NULL`, 0)
	assertGrantCount("non-admin force retry", `
SELECT COUNT(*)
FROM authz_permission_grants g
JOIN authz_roles r ON r.id = g.role_id AND r.deleted_at IS NULL
WHERE r.name <> 'qs:admin'
  AND g.resource_pattern = 'qs:evaluation:collection:assessments'
  AND g.action = 'force_retry'
  AND g.revoked_at IS NULL AND g.deleted_at IS NULL`, 0)
	assertGrantCount("retired role names", `
SELECT COUNT(*) FROM authz_roles
WHERE management_protection = 'protected'
  AND name IN ('platform:admin', 'iam:admin')
  AND deleted_at IS NULL`, 0)
	assertGrantCount("retired resource names", `
SELECT COUNT(*) FROM authz_resources
WHERE `+"`key`"+` IN ('iam:authz:collection:policies', 'iam:authz:action:check')
  AND deleted_at IS NULL`, 0)
	assertTableExists(t, db, "authz_role_inheritances", false)
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
    (id, subject_type, subject_id, role_id, granted_by, granted_at)
VALUES (?, 'user', 'migration-concurrency-user', 424242, 'migration-test', UTC_TIMESTAMP())`,
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

  AND deleted_at IS NULL`); err != nil {
		t.Fatalf("mark migrated role binding historical: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO authz_assignments
    (id, subject_type, subject_id, role_id, granted_by, granted_at)
VALUES (?, 'user', 'migration-concurrency-user', 424242, 'migration-test', UTC_TIMESTAMP())`,
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
		"authz_permission_grants",
		"authz_resources",
		"iam_role_inheritance_archives",
		"iam_role_model_migrations",
		"iam_condition_retirement_migrations",
		"iam_scope_migrations",
		"authz_roles",
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
	var scopeKinds int
	if err := db.QueryRow(`
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'authz_resources' AND COLUMN_NAME = 'scope_kinds'`, database).Scan(&scopeKinds); err != nil {
		t.Fatalf("query retired scope column: %v", err)
	}
	if scopeKinds != 0 {
		t.Fatalf("authz_resources.scope_kinds still exists after migration 27")
	}
}
