package migration

import (
	"strings"
	"testing"
)

func TestSeeddataCleanupTablesRetirementMigrationIsScopedAndFailClosed(t *testing.T) {
	up := migrationSQL(t, "000024_retire_seeddata_cleanup_tables.up.sql")
	assertSQLInOrder(t, up,
		"iam_cleanup_schema_assertion",
		"iam_cleanup_data_assertion",
		"information_schema.KEY_COLUMN_USAGE",
		"information_schema.TRIGGERS",
		"information_schema.VIEWS",
		"information_schema.ROUTINES",
		"information_schema.EVENTS",
		"information_schema.TABLE_PRIVILEGES",
		"iam_cleanup_dependency_assertion",
		"DROP TABLE IF EXISTS",
	)
	for _, required := range []string{
		"cbpt_profiles_s812v2",
		"cbpt_profile_links_s812v2",
		"cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1",
		"cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1",
		"seeddata cleanup table contents differ from verified evidence",
		"seeddata cleanup table database dependencies still exist",
	} {
		assertSQLContains(t, up, required)
	}
	for _, outOfScope := range []string{
		"auth_accounts", "auth_credentials_legacy", "children", "guardianships",
		"schema_version", "tenants", "data_dictionary", "operation_logs",
		"audit_logs", "auth_token_audit",
	} {
		assertSQLNotContains(t, up, outOfScope)
	}
	if strings.Count(strings.ToUpper(up), "DROP TABLE IF EXISTS") != 1 {
		t.Fatal("seeddata cleanup retirement must use one final DROP statement")
	}
	for _, canonicalWrite := range []string{
		"INSERT INTO PROFILES", "INSERT INTO PROFILE_LINKS",
		"UPDATE PROFILES", "UPDATE PROFILE_LINKS",
		"DELETE FROM PROFILES", "DELETE FROM PROFILE_LINKS",
	} {
		if strings.Contains(strings.ToUpper(up), canonicalWrite) {
			t.Fatalf("seeddata cleanup retirement must not write canonical data: %s", canonicalWrite)
		}
	}

	down := migrationSQL(t, "000024_retire_seeddata_cleanup_tables.down.sql")
	assertSQLContains(t, down, "SIGNAL SQLSTATE '45000'")
	assertSQLContains(t, down, "irreversible")
}
