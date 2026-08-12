package migration

import (
	"strings"
	"testing"
)

func TestLegacyAuthNTablesRetirementMigrationIsScopedAndFailClosed(t *testing.T) {
	up := migrationSQL(t, "000023_retire_legacy_authn_tables.up.sql")
	assertSQLInOrder(t, up,
		"iam_authn_schema_assertion",
		"iam_authn_account_blockers",
		"iam_authn_credential_blockers",
		"iam_authn_data_assertion",
		"information_schema.KEY_COLUMN_USAGE",
		"information_schema.TRIGGERS",
		"information_schema.VIEWS",
		"information_schema.ROUTINES",
		"information_schema.EVENTS",
		"iam_authn_dependency_assertion",
		"DROP TABLE IF EXISTS auth_credentials_legacy, auth_accounts",
	)
	for _, required := range []string{
		"auth_accounts", "auth_credentials_legacy", "auth_login_identities", "auth_credentials",
		"legacy AuthN reconciliation is incomplete",
		"legacy AuthN database dependencies still exist",
	} {
		assertSQLContains(t, up, required)
	}
	for _, outOfScope := range []string{
		"children", "guardianships", "schema_version", "tenants", "data_dictionary",
		"operation_logs", "audit_logs", "auth_token_audit",
	} {
		assertSQLNotContains(t, up, outOfScope)
	}
	if strings.Count(up, "DROP TABLE IF EXISTS") != 1 {
		t.Fatalf("AuthN retirement must use one final DROP statement")
	}
	if strings.Contains(strings.ToUpper(up), "ON DUPLICATE KEY UPDATE") {
		t.Fatal("AuthN retirement must never rewrite canonical facts")
	}

	down := migrationSQL(t, "000023_retire_legacy_authn_tables.down.sql")
	assertSQLContains(t, down, "SIGNAL SQLSTATE '45000'")
	assertSQLContains(t, down, "irreversible")
}
