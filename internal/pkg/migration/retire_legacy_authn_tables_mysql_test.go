package migration

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRetireLegacyAuthNTablesMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000023_retire_legacy_authn_tables.up.sql")
	down := migrationSQL(t, "000023_retire_legacy_authn_tables.down.sql")
	t.Cleanup(func() { resetAuthNRetirementFixture(t, db) })

	t.Run("drops converged legacy tables and tolerates already absent tables", func(t *testing.T) {
		resetAuthNRetirementFixture(t, db)
		createAuthNRetirementCanonicalFixture(t, db)
		createAuthNRetirementLegacyFixture(t, db)
		seedConvergedAuthNRetirementFixture(t, db)

		mustExecMigrationSQL(t, db, up)
		assertTableExists(t, db, "auth_accounts", false)
		assertTableExists(t, db, "auth_credentials_legacy", false)
		assertTableRowCount(t, db, "auth_login_identities", 3)
		assertTableRowCount(t, db, "auth_credentials", 1)
		mustExecMigrationSQL(t, db, up)
	})

	t.Run("partial legacy pair aborts", func(t *testing.T) {
		resetAuthNRetirementFixture(t, db)
		createAuthNRetirementCanonicalFixture(t, db)
		createAuthNRetirementLegacyFixture(t, db)
		mustExecMigrationSQL(t, db, "DROP TABLE auth_credentials_legacy")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "canonical AuthN schema or legacy table pair is incomplete") {
			t.Fatalf("migration error = %v, want schema failure", err)
		}
		assertTableExists(t, db, "auth_accounts", true)
	})

	t.Run("missing canonical fact aborts before either table is removed", func(t *testing.T) {
		resetAuthNRetirementFixture(t, db)
		createAuthNRetirementCanonicalFixture(t, db)
		createAuthNRetirementLegacyFixture(t, db)
		seedConvergedAuthNRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "DELETE FROM auth_login_identities WHERE id = 102")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "legacy AuthN reconciliation is incomplete") {
			t.Fatalf("migration error = %v, want data failure", err)
		}
		assertTableExists(t, db, "auth_accounts", true)
		assertTableExists(t, db, "auth_credentials_legacy", true)
	})

	t.Run("database dependency aborts before either table is removed", func(t *testing.T) {
		resetAuthNRetirementFixture(t, db)
		createAuthNRetirementCanonicalFixture(t, db)
		createAuthNRetirementLegacyFixture(t, db)
		seedConvergedAuthNRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "CREATE VIEW legacy_authn_view AS SELECT id FROM auth_accounts")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "legacy AuthN database dependencies still exist") {
			t.Fatalf("migration error = %v, want dependency failure", err)
		}
		assertTableExists(t, db, "auth_accounts", true)
		assertTableExists(t, db, "auth_credentials_legacy", true)
	})

	t.Run("down fails closed", func(t *testing.T) {
		if _, err := db.Exec(down); err == nil || !strings.Contains(err.Error(), "irreversible") {
			t.Fatalf("down migration error = %v, want irreversible failure", err)
		}
	})
}

func resetAuthNRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP VIEW IF EXISTS legacy_authn_view",
		"DROP TABLE IF EXISTS auth_credentials_legacy",
		"DROP TABLE IF EXISTS auth_accounts",
		"DROP TABLE IF EXISTS auth_credentials",
		"DROP TABLE IF EXISTS auth_login_identities",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset AuthN retirement fixture with %q: %v", statement, err)
		}
	}
}

func createAuthNRetirementCanonicalFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExecMigrationSQL(t, db, `CREATE TABLE auth_login_identities (
id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, provider VARCHAR(32) NOT NULL,
realm VARCHAR(128) NOT NULL DEFAULT '', identifier VARCHAR(255) NOT NULL, global_identifier VARCHAR(255) DEFAULT NULL,
status VARCHAR(32) NOT NULL, linked_at DATETIME NOT NULL, PRIMARY KEY (id),
UNIQUE KEY uk_provider_realm_identifier (provider, realm, identifier)) ENGINE=InnoDB;
CREATE TABLE auth_credentials (
id BIGINT UNSIGNED NOT NULL, login_identity_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
material VARBINARY(4096) DEFAULT NULL, algo VARCHAR(64) DEFAULT NULL, status VARCHAR(32) NOT NULL,
PRIMARY KEY (id), UNIQUE KEY uk_identity_type (login_identity_id, type)) ENGINE=InnoDB`)
}

func createAuthNRetirementLegacyFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExecMigrationSQL(t, db, `CREATE TABLE auth_accounts (
id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
app_id VARCHAR(64) NOT NULL DEFAULT '', external_id VARCHAR(128) NOT NULL, unique_id VARCHAR(128) DEFAULT NULL,
scoped_tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0, status TINYINT NOT NULL DEFAULT 1,
PRIMARY KEY (id), UNIQUE KEY uk_type_app_external (type, app_id, external_id)) ENGINE=InnoDB;
CREATE TABLE auth_credentials_legacy (
id BIGINT NOT NULL AUTO_INCREMENT, account_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
idp VARCHAR(32) DEFAULT NULL, idp_identifier VARCHAR(256) NOT NULL DEFAULT '', app_id VARCHAR(64) DEFAULT NULL,
material VARBINARY(512) DEFAULT NULL, algo VARCHAR(32) DEFAULT NULL, status TINYINT NOT NULL DEFAULT 1,
PRIMARY KEY (id), UNIQUE KEY uk_account_type (account_id, type)) ENGINE=InnoDB`)
}

func seedConvergedAuthNRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExecMigrationSQL(t, db, `INSERT INTO auth_accounts
(id, user_id, type, app_id, external_id, unique_id, scoped_tenant_id, status) VALUES
(10, 100, 'opera', '', 'operator', NULL, 0, 1),
(11, 101, 'wc-minip', 'wx-app', 'openid', 'unionid', 0, 1);
INSERT INTO auth_credentials_legacy
(id, account_id, type, idp, idp_identifier, app_id, material, algo, status) VALUES
(20, 10, 'password', NULL, '', NULL, 'legacy-password-hash', 'bcrypt', 1),
(21, 10, 'phone_otp', 'phone', '+8613800000000', NULL, NULL, NULL, 1),
(22, 11, 'oauth_wx_minip', 'wechat', 'unionid', 'wx-app', NULL, NULL, 1),
(23, 999, 'password', NULL, '', NULL, 'unreachable-password', 'bcrypt', 1),
(24, 999, 'oauth_wx_minip', 'wechat', 'unreachable', 'wx-app', NULL, NULL, 1);
INSERT INTO auth_login_identities
(id, user_id, provider, realm, identifier, global_identifier, status, linked_at) VALUES
(100, 100, 'username', 'default', 'operator', NULL, 'active', NOW()),
(101, 100, 'phone', 'global', '+8613800000000', NULL, 'active', NOW()),
(102, 101, 'wechat_minip', 'wx-app', 'openid', 'unionid', 'active', NOW());
INSERT INTO auth_credentials
(id, login_identity_id, type, material, algo, status) VALUES
(200, 100, 'password', 'canonical-password-hash', 'bcrypt', 'enabled')`)
}
