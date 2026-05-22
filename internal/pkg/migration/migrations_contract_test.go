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

func TestAuthNLoginIdentityMigrationUsesIdempotentCreateTableGuards(t *testing.T) {
	sql := migrationSQL(t, "000001_init_schema.up.sql")

	assertSQLContains(t, sql, "auth_login_identities")
	assertSQLContains(t, sql, "auth_credentials")
	assertSQLContains(t, sql, "`uk_provider_realm_identifier`")
	assertSQLContains(t, sql, "`uk_identity_type`")
}

func TestAuthNLoginIdentitySchemaMigrationPreservesLegacyCredentialTable(t *testing.T) {
	sql := migrationSQL(t, "000011_ensure_authn_login_identity_schema.up.sql")

	assertSQLContains(t, sql, "information_schema.TABLES")
	assertSQLContains(t, sql, "information_schema.COLUMNS")
	assertSQLContains(t, sql, "auth_credentials_legacy")
	assertSQLContains(t, sql, "RENAME TABLE `auth_credentials` TO `auth_credentials_legacy`")
	assertSQLContains(t, sql, "CREATE TABLE IF NOT EXISTS `auth_login_identities`")
	assertSQLContains(t, sql, "CREATE TABLE IF NOT EXISTS `auth_credentials`")
	assertSQLContains(t, sql, "`login_identity_id`")
	assertSQLContains(t, sql, "`uk_provider_realm_identifier`")
	assertSQLContains(t, sql, "`uk_identity_type`")
	assertSQLContains(t, sql, "PREPARE iam_stmt FROM @iam_sql")
	assertSQLContains(t, sql, "EXECUTE iam_stmt")
	assertSQLContains(t, sql, "DEALLOCATE PREPARE iam_stmt")
}

func TestAuthZResourceKeyMigrationMapsCatalogAndPolicyFacts(t *testing.T) {
	upSQL := migrationSQL(t, "000012_authz_four_segment_resource_keys.up.sql")
	downSQL := migrationSQL(t, "000012_authz_four_segment_resource_keys.down.sql")

	assertSQLContains(t, upSQL, "UPDATE `authz_resources`")
	assertSQLContains(t, upSQL, "UPDATE `casbin_rule`")
	assertSQLContains(t, upSQL, "WHEN 'iam:users' THEN 'iam:identity:collection:users'")
	assertSQLContains(t, upSQL, "WHEN 'qs:*' THEN 'qs:*:*:*'")
	assertSQLContains(t, upSQL, "WHEN 'qs:codes' THEN 'qs:code:collection:codes'")

	assertSQLContains(t, downSQL, "WHEN 'iam:identity:collection:users' THEN 'iam:users'")
	assertSQLContains(t, downSQL, "WHEN 'qs:*:*:*' THEN 'qs:*'")
	assertSQLContains(t, downSQL, "WHEN 'qs:code:collection:codes' THEN 'qs:codes'")
}

func TestLegacyAuthZResourceKeyMigrationMapsIdentityAliasesAndTenantDomain(t *testing.T) {
	upSQL := migrationSQL(t, "000014_legacy_authz_resource_keys_and_domain.up.sql")
	downSQL := migrationSQL(t, "000014_legacy_authz_resource_keys_and_domain.down.sql")

	assertSQLContains(t, upSQL, "WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'")
	assertSQLContains(t, upSQL, "WHEN 'iam:children' THEN 'iam:identity:collection:profiles'")
	assertSQLContains(t, upSQL, "WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'")
	assertSQLContains(t, upSQL, "UPDATE `casbin_rule`")
	assertSQLContains(t, upSQL, "SET `v1` = 'fangcun'")
	assertSQLContains(t, upSQL, "SET `v2` = 'fangcun'")
	assertSQLContains(t, upSQL, "WHERE `tenant_id` = '1'")

	assertSQLContains(t, downSQL, "WHEN 'iam:authn:collection:login_identities' THEN 'iam:accounts'")
	assertSQLContains(t, downSQL, "SET `v1` = '1'")
	assertSQLContains(t, downSQL, "SET `v2` = '1'")
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
