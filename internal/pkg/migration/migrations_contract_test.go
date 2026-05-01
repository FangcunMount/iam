package migration

import (
	"strings"
	"testing"
)

func TestLegacyUCTablesAreBridgedBeforeProfileLinkIndex(t *testing.T) {
	sql := migrationSQL(t, "000002_add_profile_links_profile_id_index.up.sql")

	assertSQLInOrder(t, sql,
		"CREATE TABLE IF NOT EXISTS `profiles`",
		"CREATE TABLE IF NOT EXISTS `profile_links`",
		"FROM `children`",
		"FROM `guardianships`",
		"CREATE INDEX `idx_profile_id` ON `profile_links` (`profile_id`)",
	)
	assertSQLContains(t, sql, "information_schema.TABLES")
	assertSQLContains(t, sql, "ON DUPLICATE KEY UPDATE")

	assertSQLNotContains(t, sql, "DROP TABLE `children`")
	assertSQLNotContains(t, sql, "DROP TABLE `guardianships`")
}

func TestSchemaDriftMigrationsUseIdempotentDDLGuards(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		column     string
		index      string
		needsIndex bool
	}{
		{
			name:       "auth account scoped tenant",
			file:       "000004_add_auth_accounts_scoped_tenant.up.sql",
			column:     "`scoped_tenant_id`",
			index:      "`idx_scoped_tenant_id`",
			needsIndex: true,
		},
		{
			name:       "profile link self key",
			file:       "000007_add_active_self_profile_link_guard.up.sql",
			column:     "`self_key`",
			index:      "`uk_active_self_profile_link`",
			needsIndex: true,
		},
		{
			name:   "authz scope kinds",
			file:   "000008_add_authz_scope.up.sql",
			column: "`scope_kinds`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := migrationSQL(t, tt.file)
			assertSQLContains(t, sql, "information_schema.COLUMNS")
			assertSQLContains(t, sql, tt.column)
			assertSQLContains(t, sql, "PREPARE iam_stmt FROM @iam_sql")
			assertSQLContains(t, sql, "EXECUTE iam_stmt")
			assertSQLContains(t, sql, "DEALLOCATE PREPARE iam_stmt")
			if tt.needsIndex {
				assertSQLContains(t, sql, "information_schema.STATISTICS")
				assertSQLContains(t, sql, tt.index)
			}
		})
	}
}

func migrationSQL(t *testing.T, name string) string {
	t.Helper()

	content, err := migrations.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(content)
}

func assertSQLContains(t *testing.T, sql, fragment string) {
	t.Helper()

	if !strings.Contains(sql, fragment) {
		t.Fatalf("migration does not contain %q", fragment)
	}
}

func assertSQLNotContains(t *testing.T, sql, fragment string) {
	t.Helper()

	if strings.Contains(sql, fragment) {
		t.Fatalf("migration unexpectedly contains %q", fragment)
	}
}

func assertSQLInOrder(t *testing.T, sql string, fragments ...string) {
	t.Helper()

	offset := 0
	for _, fragment := range fragments {
		index := strings.Index(sql[offset:], fragment)
		if index < 0 {
			t.Fatalf("migration does not contain %q after offset %d", fragment, offset)
		}
		offset += index + len(fragment)
	}
}
