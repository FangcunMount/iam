package migration

import (
	"strings"
	"testing"
)

func TestSchemaVersionRetirementMigrationIsScopedAndFailClosed(t *testing.T) {
	up := migrationSQL(t, "000020_retire_schema_version.up.sql")
	assertSQLInOrder(t, up,
		"information_schema.KEY_COLUMN_USAGE",
		"information_schema.TRIGGERS",
		"information_schema.VIEWS",
		"information_schema.ROUTINES",
		"information_schema.EVENTS",
		"iam_schema_version_retirement_assertion",
		"DROP TABLE IF EXISTS schema_version",
	)
	for _, outOfScope := range []string{
		"auth_accounts", "auth_credentials_legacy", "tenants", "data_dictionary",
		"operation_logs", "audit_logs", "auth_token_audit",
	} {
		assertSQLNotContains(t, up, outOfScope)
	}
	if strings.Count(up, "DROP TABLE IF EXISTS") != 1 {
		t.Fatalf("schema_version retirement must use one final DROP statement")
	}
	down := migrationSQL(t, "000020_retire_schema_version.down.sql")
	assertSQLContains(t, down, "SIGNAL SQLSTATE '45000'")
	assertSQLContains(t, down, "irreversible")
}
