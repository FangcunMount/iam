package migration

import (
	"database/sql"
	"strings"
	"testing"
)

func TestAuthNGlobalIdentifierUniqueGuardMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000028_authn_global_identifier_unique_guard.up.sql")
	down := migrationSQL(t, "000028_authn_global_identifier_unique_guard.down.sql")
	t.Cleanup(func() { resetAuthNGlobalIdentifierFixture(t, db) })

	t.Run("deduplicates same owner and enforces one canonical row", func(t *testing.T) {
		resetAuthNGlobalIdentifierFixture(t, db)
		createAuthNGlobalIdentifierFixture(t, db)
		insertAuthNGlobalIdentity(t, db, 1, 100, "wx-app-a", "openid-a", "union-1", "active", "2026-01-01 00:00:00")
		insertAuthNGlobalIdentity(t, db, 2, 100, "wx-app-b", "openid-b", "union-1", "active", "2026-01-02 00:00:00")

		mustExecMigrationSQL(t, db, up)
		assertAuthNGlobalIdentifierIndex(t, db, true)
		assertGlobalIdentifierValue(t, db, 1, "union-1")
		assertGlobalIdentifierValue(t, db, 2, "")

		if _, err := db.Exec(`
INSERT INTO auth_login_identities
    (id, user_id, provider, realm, identifier, global_identifier, status, linked_at, created_at)
VALUES (3, 100, 'wechat_minip', 'wx-app-c', 'openid-c', 'union-1', 'active', NOW(), NOW())`); err == nil {
			t.Fatal("unique guard accepted a duplicate same-User global identifier")
		}
		if _, err := db.Exec(`
INSERT INTO auth_login_identities
    (id, user_id, provider, realm, identifier, global_identifier, status, linked_at, created_at)
VALUES (3, 100, 'wechat_minip', 'wx-app-c', 'openid-c', NULL, 'active', NOW(), NOW())`); err != nil {
			t.Fatalf("unique guard rejected a realm row without duplicate global identifier: %v", err)
		}
		if _, err := db.Exec(`
INSERT INTO auth_login_identities
    (id, user_id, provider, realm, identifier, global_identifier, status, linked_at, created_at)
VALUES (4, 200, 'wechat_minip', 'wx-app-d', 'openid-d', 'union-1', 'active', NOW(), NOW())`); err == nil {
			t.Fatal("unique guard accepted a different-User global identifier")
		}

		mustExecMigrationSQL(t, db, down)
		assertAuthNGlobalIdentifierIndex(t, db, false)
		mustExecMigrationSQL(t, db, up)
		assertAuthNGlobalIdentifierIndex(t, db, true)
	})

	t.Run("conflicting historical owners fail before data or schema changes", func(t *testing.T) {
		resetAuthNGlobalIdentifierFixture(t, db)
		createAuthNGlobalIdentifierFixture(t, db)
		insertAuthNGlobalIdentity(t, db, 1, 100, "wx-app-a", "openid-a", "union-1", "active", "2026-01-01 00:00:00")
		insertAuthNGlobalIdentity(t, db, 2, 200, "wx-app-b", "openid-b", "union-1", "active", "2026-01-02 00:00:00")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "AuthN global identifier preflight failed") {
			t.Fatalf("migration error = %v, want owner-conflict failure", err)
		}
		assertAuthNGlobalIdentifierIndex(t, db, false)
		assertGlobalIdentifierValue(t, db, 1, "union-1")
		assertGlobalIdentifierValue(t, db, 2, "union-1")
	})

	t.Run("noncanonical historical identifier fails before data or schema changes", func(t *testing.T) {
		resetAuthNGlobalIdentifierFixture(t, db)
		createAuthNGlobalIdentifierFixture(t, db)
		insertAuthNGlobalIdentity(t, db, 1, 100, "wx-app-a", "openid-a", " union-1 ", "active", "2026-01-01 00:00:00")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "AuthN global identifier preflight failed") {
			t.Fatalf("migration error = %v, want noncanonical-identifier failure", err)
		}
		assertAuthNGlobalIdentifierIndex(t, db, false)
		assertGlobalIdentifierValue(t, db, 1, " union-1 ")
	})
}

func resetAuthNGlobalIdentifierFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DROP TABLE IF EXISTS `auth_login_identities`"); err != nil {
		t.Fatalf("reset AuthN global-identifier fixture: %v", err)
	}
}

func createAuthNGlobalIdentifierFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExecMigrationSQL(t, db, `
CREATE TABLE auth_login_identities (
  id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  provider VARCHAR(32) NOT NULL,
  realm VARCHAR(128) NOT NULL DEFAULT '',
  identifier VARCHAR(255) NOT NULL,
  global_identifier VARCHAR(255) DEFAULT NULL,
  status VARCHAR(32) NOT NULL,
  linked_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_provider_realm_identifier (provider, realm, identifier),
  KEY idx_global_identifier (global_identifier)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
}

func insertAuthNGlobalIdentity(
	t *testing.T,
	db *sql.DB,
	id, userID int,
	realm, identifier, globalIdentifier, status, linkedAt string,
) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO auth_login_identities
    (id, user_id, provider, realm, identifier, global_identifier, status, linked_at, created_at)
VALUES (?, ?, 'wechat_minip', ?, ?, ?, ?, ?, ?)`, id, userID, realm, identifier, globalIdentifier, status, linkedAt, linkedAt); err != nil {
		t.Fatalf("insert AuthN global identity %d: %v", id, err)
	}
}

func assertAuthNGlobalIdentifierIndex(t *testing.T, db *sql.DB, want bool) {
	t.Helper()
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))
	var indexes int
	if err := db.QueryRow(`
SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'auth_login_identities'
  AND INDEX_NAME = 'uk_auth_login_identities_global'`, database).Scan(&indexes); err != nil {
		t.Fatalf("query global-identifier unique index: %v", err)
	}
	wantCount := 0
	if want {
		wantCount = 1
	}
	if indexes != wantCount {
		t.Fatalf("global-identifier unique index count = %d, want %d", indexes, wantCount)
	}
}

func assertGlobalIdentifierValue(t *testing.T, db *sql.DB, id int, want string) {
	t.Helper()
	var value sql.NullString
	if err := db.QueryRow("SELECT global_identifier FROM auth_login_identities WHERE id = ?", id).Scan(&value); err != nil {
		t.Fatalf("query global identifier for id %d: %v", id, err)
	}
	got := ""
	if value.Valid {
		got = value.String
	}
	if got != want {
		t.Fatalf("global identifier for id %d = %q, want %q", id, got, want)
	}
}
