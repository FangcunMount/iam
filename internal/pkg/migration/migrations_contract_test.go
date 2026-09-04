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
	assertSQLContains(t, upSQL, "000014-v4: MySQL-safe")
	assertSQLNotContains(t, upSQL, "EXISTS (")
	assertSQLContains(t, upSQL, "DELETE `legacy` FROM `authz_resources`")
	assertSQLContains(t, upSQL, "DELETE `later` FROM `casbin_rule`")
	assertSQLContains(t, upSQL, "DELETE `legacy` FROM `casbin_rule`")
	assertSQLContains(t, upSQL, "DELETE `old` FROM `authz_policy_versions`")
	assertSQLContains(t, upSQL, "DELETE `dup` FROM `authz_roles`")
	assertSQLContains(t, upSQL, "DELETE `dup` FROM `casbin_rule`")

	assertSQLContains(t, downSQL, "WHEN 'iam:authn:collection:login_identities' THEN 'iam:accounts'")
	assertSQLContains(t, downSQL, "SET `v1` = '1'")
	assertSQLContains(t, downSQL, "SET `v2` = '1'")
}

func TestNormTableMigrationRegistersCatalogWithoutFreshLegacyPolicy(t *testing.T) {
	upSQL := migrationSQL(t, "000015_add_norm_table_content_manager_policy.up.sql")
	downSQL := migrationSQL(t, "000015_add_norm_table_content_manager_policy.down.sql")

	assertSQLContains(t, upSQL, "qs:modelcatalog:collection:norm_tables")
	assertSQLContains(t, upSQL, "JSON_ARRAY('read', 'list', 'import')")
	assertSQLNotContains(t, upSQL, "INSERT INTO `casbin_rule`")
	assertSQLContains(t, downSQL, "DELETE FROM `casbin_rule`")
	assertSQLContains(t, downSQL, "DELETE FROM `authz_resources`")
}

func TestJWKSSingleActiveGuardMigrationUsesGeneratedUniqueSlot(t *testing.T) {
	up := migrationSQL(t, "000016_jwks_single_active_guard.up.sql")
	for _, fragment := range []string{
		"ADD COLUMN `active_guard` TINYINT",
		"CASE WHEN `status` = 1 THEN 1 ELSE NULL END",
		"ADD UNIQUE INDEX `uk_jwks_keys_single_active` (`active_guard`)",
	} {
		if !strings.Contains(up, fragment) {
			t.Fatalf("jwks single-active migration missing %q", fragment)
		}
	}

	down := migrationSQL(t, "000016_jwks_single_active_guard.down.sql")
	for _, fragment := range []string{
		"DROP INDEX `uk_jwks_keys_single_active`",
		"DROP COLUMN `active_guard`",
	} {
		if !strings.Contains(down, fragment) {
			t.Fatalf("jwks single-active down migration missing %q", fragment)
		}
	}
}

func TestIdentityConsistencyMigrations(t *testing.T) {
	phoneGuard := migrationSQL(t, "000017_users_active_phone_unique_guard.up.sql")
	for _, fragment := range []string{
		"ADD COLUMN `active_phone` VARCHAR(20)",
		"`deleted_at` IS NULL",
		"`phone` <> ''",
		"ADD UNIQUE INDEX `uk_users_active_phone` (`active_phone`)",
	} {
		assertSQLContains(t, phoneGuard, fragment)
	}
	phoneDown := migrationSQL(t, "000017_users_active_phone_unique_guard.down.sql")
	assertSQLContains(t, phoneDown, "DROP INDEX `uk_users_active_phone`")
	assertSQLContains(t, phoneDown, "DROP COLUMN `active_phone`")

	revocation := migrationSQL(t, "000018_identity_session_revocation_outbox.up.sql")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS `identity_session_revocation_outbox`",
		"`user_version` INT UNSIGNED NOT NULL",
		"`status` VARCHAR(16) NOT NULL DEFAULT 'pending'",
		"`next_attempt_at` DATETIME(3) NOT NULL",
		"UNIQUE KEY `uk_identity_session_revocation_version_action`",
	} {
		assertSQLContains(t, revocation, fragment)
	}
}

func TestAuthZActiveRoleBindingGuardMigration(t *testing.T) {
	up := migrationSQL(t, "000025_authz_active_rolebinding_guard.up.sql")
	for _, fragment := range []string{
		"information_schema.COLUMNS",
		"ADD COLUMN `active_guard` TINYINT GENERATED ALWAYS AS",
		"CASE WHEN `deleted_at` IS NULL THEN 1 ELSE NULL END",
		"information_schema.STATISTICS",
		"CREATE UNIQUE INDEX `uk_authz_assignments_active`",
		"(`subject_type`, `subject_id`, `role_id`, `tenant_id`, `active_guard`)",
	} {
		assertSQLContains(t, up, fragment)
	}
	assertSQLNotContains(t, up, "DELETE FROM `authz_assignments`")

	down := migrationSQL(t, "000025_authz_active_rolebinding_guard.down.sql")
	assertSQLContains(t, down, "DROP INDEX `uk_authz_assignments_active`")
	assertSQLContains(t, down, "DROP COLUMN `active_guard`")
}

func TestAuthZPermissionGrantExpansionMigrationIsAdditiveAndGuarded(t *testing.T) {
	up := migrationSQL(t, "000026_add_authz_permission_grants.up.sql")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS `authz_permission_grants`",
		"`constraint_set`   JSON            NOT NULL",
		"`grant_key`        CHAR(64)        NOT NULL",
		"UNIQUE KEY `uk_authz_permission_grants_active` (`grant_key`, `active_guard`)",
		"CREATE TABLE IF NOT EXISTS `authz_role_inheritances`",
		"UNIQUE KEY `uk_authz_role_inheritances_active`",
		"CREATE TABLE IF NOT EXISTS `authz_cutover_state`",
		"ADD COLUMN `attribute_schema` JSON NULL",
		"information_schema.COLUMNS",
		"WHERE `key` = 'iam:identity:instance:profile'",
		"AND `app_name` = 'iam'",
		"AND `domain` = 'uc'",
		"AND `type` = 'instance'",
		"SET `domain` = 'identity'",
		"UPDATE `authz_assignments` AS `stale_assignment`",
		"902000001 AS `stale_assignment_id`",
		"SELECT 902000006, 'user', '110002', 900000102, 'fangcun', 'qs:content_manager'",
		"613485615136125486 AS `current_role_id`",
		"613500091088515630 AS `current_assignment_id`",
		"LEFT JOIN `authz_roles` AS `stale_role`",
		"INNER JOIN `authz_roles` AS `current_role`",
		"INNER JOIN `authz_assignments` AS `current_assignment`",
		"AND `stale_role`.`id` IS NULL",
		"SET `stale_assignment`.`deleted_at` = CURRENT_TIMESTAMP(3)",
	} {
		assertSQLContains(t, up, fragment)
	}
	for _, forbidden := range []string{
		"INSERT INTO `authz_permission_grants`",
		"UPDATE `casbin_rule`",
		"DELETE FROM `casbin_rule`",
		"DROP TABLE `casbin_rule`",
		"DELETE FROM `authz_assignments`",
	} {
		assertSQLNotContains(t, up, forbidden)
	}

	down := migrationSQL(t, "000026_add_authz_permission_grants.down.sql")
	assertSQLContains(t, down, "DROP TABLE IF EXISTS `authz_permission_grants`")
	assertSQLContains(t, down, "DROP TABLE IF EXISTS `authz_role_inheritances`")
	assertSQLContains(t, down, "DROP TABLE IF EXISTS `authz_cutover_state`")
	assertSQLContains(t, down, "DROP COLUMN `attribute_schema`")
}

func TestAuthNGlobalIdentifierUniqueGuardMigration(t *testing.T) {
	up := migrationSQL(t, "000028_authn_global_identifier_unique_guard.up.sql")
	for _, fragment := range []string{
		"iam_authn_global_identifier_conflicts",
		"COUNT(DISTINCT `user_id`) > 1",
		"ROW_NUMBER() OVER",
		"CASE WHEN `status` = 'active' THEN 0 ELSE 1 END",
		"UPDATE `auth_login_identities`",
		"SET `identity`.`global_identifier` = NULL",
		"ADD UNIQUE KEY `uk_auth_login_identities_global`",
	} {
		assertSQLContains(t, up, fragment)
	}
	assertSQLNotContains(t, up, "DELETE FROM `auth_login_identities`")
	assertSQLNotContains(t, up, "CREATE TABLE `auth_login_global_identifier_owners`")

	down := migrationSQL(t, "000028_authn_global_identifier_unique_guard.down.sql")
	assertSQLContains(t, down, "DROP INDEX `uk_auth_login_identities_global`")
}

func TestJWKSGraceActionRetirementUsesCanonicalResourceKey(t *testing.T) {
	up := migrationSQL(t, "000029_retire_jwks_enter_grace_action.up.sql")
	down := migrationSQL(t, "000029_retire_jwks_enter_grace_action.down.sql")

	for _, migration := range []string{up, down} {
		assertSQLContains(t, migration, "UPDATE `authz_resources`")
		assertSQLContains(t, migration, "WHERE `key` = 'iam:authn:collection:jwks'")
		assertSQLNotContains(t, migration, "resource_key")
	}
	assertSQLContains(t, up, "JSON_REMOVE")
	assertSQLContains(t, down, "JSON_ARRAY_APPEND")
}

func TestAuthZLegacyRetirementRequiresEvidenceAndIsIrreversible(t *testing.T) {
	up := migrationSQL(t, "000027_retire_legacy_authz.up.sql")
	for _, fragment := range []string{
		"authz_cutover_state",
		"evidence_hash",
		"iam_authz_cutover_verification_is_required",
		"DROP TABLE `casbin_rule`",
		"DROP COLUMN `scope_kinds`",
		"DROP TABLE `authz_cutover_state`",
	} {
		assertSQLContains(t, up, fragment)
	}
	down := migrationSQL(t, "000027_retire_legacy_authz.down.sql")
	assertSQLContains(t, down, "SIGNAL SQLSTATE '45000'")
	assertSQLContains(t, down, "restore the pre-cutover backup")
}

func TestRetiredIdentityTablesMigrationBackfillsAndValidatesBeforeDrop(t *testing.T) {
	up := migrationSQL(t, "000019_retire_legacy_tables.up.sql")

	assertSQLInOrder(t, up,
		"CREATE TABLE IF NOT EXISTS profiles",
		"INSERT IGNORE INTO profiles",
		"FROM children",
		"INSERT IGNORE INTO profile_links",
		"FROM guardianships",
		"iam_children_mismatches",
		"iam_guardianship_mismatches",
		"iam_retirement_dependencies",
		"DROP TABLE IF EXISTS",
	)
	assertSQLNotContains(t, up, "ON DUPLICATE KEY UPDATE")
	for _, table := range []string{
		"children",
		"guardianships",
	} {
		assertSQLContains(t, up, table)
	}
	for _, outOfScopeTable := range []string{
		"auth_accounts",
		"auth_credentials_legacy",
		"schema_version",
		"tenants",
		"data_dictionary",
		"operation_logs",
		"audit_logs",
		"auth_token_audit",
	} {
		if strings.Contains(up, outOfScopeTable) {
			t.Fatalf("retirement migration must not include out-of-scope table %q", outOfScopeTable)
		}
	}
	for _, guard := range []string{
		"information_schema.KEY_COLUMN_USAGE",
		"information_schema.TRIGGERS",
		"information_schema.VIEWS",
		"information_schema.ROUTINES",
		"information_schema.EVENTS",
		"iam_identity_retirement_assertion",
		"legacy Identity parity is incomplete",
		"iam_identity_dependency_assertion",
		"legacy table database dependencies still exist",
	} {
		assertSQLContains(t, up, guard)
	}
	if strings.Count(up, "DROP TABLE IF EXISTS") != 1 {
		t.Fatalf("retirement migration must use one final DROP statement, got %d", strings.Count(up, "DROP TABLE IF EXISTS"))
	}

	down := migrationSQL(t, "000019_retire_legacy_tables.down.sql")
	assertSQLContains(t, down, "SIGNAL SQLSTATE '45000'")
	assertSQLContains(t, down, "irreversible")
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
