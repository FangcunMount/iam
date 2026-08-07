package migration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUnusedPlatformRetirementMigrationIsScopedAndFailClosed(t *testing.T) {
	up := migrationSQL(t, "000021_retire_unused_platform_tables.up.sql")
	assertSQLInOrder(t, up,
		"information_schema.KEY_COLUMN_USAGE",
		"information_schema.TRIGGERS",
		"information_schema.VIEWS",
		"information_schema.ROUTINES",
		"information_schema.EVENTS",
		"iam_platform_retirement_assertion",
		"DROP TABLE IF EXISTS tenants, data_dictionary",
	)
	for _, required := range []string{"tenants", "data_dictionary"} {
		assertSQLContains(t, up, required)
	}
	for _, outOfScope := range []string{
		"auth_accounts", "auth_credentials_legacy", "schema_version",
		"operation_logs", "audit_logs", "auth_token_audit",
	} {
		assertSQLNotContains(t, up, outOfScope)
	}
	if strings.Count(up, "DROP TABLE IF EXISTS") != 1 {
		t.Fatalf("platform retirement must use one final DROP statement")
	}
	down := migrationSQL(t, "000021_retire_unused_platform_tables.down.sql")
	assertSQLContains(t, down, "SIGNAL SQLSTATE '45000'")
	assertSQLContains(t, down, "irreversible")
}

func TestCurrentBootstrapDoesNotReferenceRetiredPlatformSchema(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	bootstrapPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "configs", "mysql", "bootstrap.sql")
	content, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("read bootstrap SQL: %v", err)
	}
	bootstrap := string(content)
	for _, forbidden := range []string{"`tenants`", "`data_dictionary`", "`id_card`", "USE `iam`"} {
		if strings.Contains(bootstrap, forbidden) {
			t.Fatalf("current bootstrap still references retired schema token %q", forbidden)
		}
	}
}
