package migration

import (
	"strings"
	"testing"
)

func TestUnusedAuditTablesRetirementMigrationIsScopedAndFailClosed(t *testing.T) {
	up := migrationSQL(t, "000022_retire_unused_audit_tables.up.sql")
	assertSQLInOrder(t, up,
		"information_schema.KEY_COLUMN_USAGE",
		"information_schema.TRIGGERS",
		"information_schema.VIEWS",
		"information_schema.ROUTINES",
		"information_schema.EVENTS",
		"iam_audit_retirement_assertion",
		"DROP TABLE IF EXISTS operation_logs, audit_logs, auth_token_audit",
	)
	for _, required := range []string{"operation_logs", "audit_logs", "auth_token_audit"} {
		assertSQLContains(t, up, required)
	}
	for _, outOfScope := range []string{
		"auth_accounts", "auth_credentials_legacy", "schema_version",
		"tenants", "data_dictionary", "children", "guardianships",
	} {
		assertSQLNotContains(t, up, outOfScope)
	}
	if strings.Count(up, "DROP TABLE IF EXISTS") != 1 {
		t.Fatalf("audit table retirement must use one final DROP statement")
	}
	down := migrationSQL(t, "000022_retire_unused_audit_tables.down.sql")
	assertSQLContains(t, down, "SIGNAL SQLSTATE '45000'")
	assertSQLContains(t, down, "irreversible")
}
