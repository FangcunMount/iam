package migration

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestIdentityConsistencyMigrationsMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS `identity_session_revocation_outbox`")
		_, _ = db.Exec("DROP TABLE IF EXISTS `domain_event_outbox`")
		_, _ = db.Exec("DROP TABLE IF EXISTS `users`")
	})

	t.Run("active phone guard", func(t *testing.T) {
		up := migrationSQL(t, "000017_users_active_phone_unique_guard.up.sql")
		down := migrationSQL(t, "000017_users_active_phone_unique_guard.down.sql")

		t.Run("up down up preserves users and permits soft-delete reuse", func(t *testing.T) {
			resetLegacyUsersTable(t, db)
			insertMigrationUser(t, db, 1, "+8613800138000", false)
			insertMigrationUser(t, db, 2, "+8613900139000", false)
			rowsBefore := migrationUserRows(t, db)

			mustExecMigrationSQL(t, db, up)
			assertActivePhoneGuardSchema(t, db, true)
			if got := migrationUserRows(t, db); got != rowsBefore {
				t.Fatalf("up migration changed users: got %q want %q", got, rowsBefore)
			}
			if _, err := db.Exec(
				"INSERT INTO `users` (`id`, `phone`) VALUES (?, ?)",
				3,
				"+8613800138000",
			); err == nil {
				t.Fatal("active-phone guard accepted a duplicate active phone")
			}
			if _, err := db.Exec("UPDATE `users` SET `deleted_at` = UTC_TIMESTAMP(3) WHERE `id` = 1"); err != nil {
				t.Fatalf("soft delete user: %v", err)
			}
			insertMigrationUser(t, db, 3, "+8613800138000", false)

			rowsBeforeDown := migrationUserRows(t, db)
			mustExecMigrationSQL(t, db, down)
			assertActivePhoneGuardSchema(t, db, false)
			if got := migrationUserRows(t, db); got != rowsBeforeDown {
				t.Fatalf("down migration changed users: got %q want %q", got, rowsBeforeDown)
			}
			mustExecMigrationSQL(t, db, up)
			assertActivePhoneGuardSchema(t, db, true)
		})

		t.Run("legacy duplicate fails without partial schema", func(t *testing.T) {
			resetLegacyUsersTable(t, db)
			insertMigrationUser(t, db, 1, "+8613800138000", false)
			insertMigrationUser(t, db, 2, "+8613800138000", false)
			rowsBefore := migrationUserRows(t, db)

			if _, err := db.Exec(up); err == nil {
				t.Fatal("active-phone migration succeeded with duplicate legacy phones")
			}
			assertActivePhoneGuardSchema(t, db, false)
			if got := migrationUserRows(t, db); got != rowsBefore {
				t.Fatalf("failed migration changed users: got %q want %q", got, rowsBefore)
			}
		})
	})

	t.Run("session revocation outbox up down up", func(t *testing.T) {
		resetLegacyUsersTable(t, db)
		resetLegacyDomainOutbox(t, db)
		up := migrationSQL(t, "000018_identity_session_revocation_outbox.up.sql")
		down := migrationSQL(t, "000018_identity_session_revocation_outbox.down.sql")

		mustExecMigrationSQL(t, db, up)
		assertTableExists(t, db, "identity_session_revocation_outbox", true)
		if _, err := db.Exec(`
INSERT INTO identity_session_revocation_outbox
  (user_id, user_version, action, reason)
VALUES (?, ?, ?, ?)`, 1, 2, "revoke_all", "user_blocked"); err != nil {
			t.Fatalf("insert session revocation task: %v", err)
		}
		if _, err := db.Exec(`
INSERT INTO identity_session_revocation_outbox
  (user_id, user_version, action, reason)
VALUES (?, ?, ?, ?)`, 1, 2, "revoke_all", "user_deactivated"); err == nil {
			t.Fatal("session revocation outbox accepted a duplicate version/action")
		}

		mustExecMigrationSQL(t, db, down)
		assertTableExists(t, db, "identity_session_revocation_outbox", false)
		assertTableRowCount(t, db, "users", 1)
		assertTableRowCount(t, db, "domain_event_outbox", 1)

		mustExecMigrationSQL(t, db, up)
		assertTableExists(t, db, "identity_session_revocation_outbox", true)
	})
}

func resetLegacyUsersTable(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DROP TABLE IF EXISTS `identity_session_revocation_outbox`"); err != nil {
		t.Fatalf("drop session revocation outbox: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS `users`"); err != nil {
		t.Fatalf("drop users: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE users (
  id BIGINT UNSIGNED NOT NULL,
  phone VARCHAR(20) NOT NULL DEFAULT '',
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_phone (phone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		t.Fatalf("create legacy users: %v", err)
	}
}

func resetLegacyDomainOutbox(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DROP TABLE IF EXISTS `domain_event_outbox`"); err != nil {
		t.Fatalf("drop domain_event_outbox: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE domain_event_outbox (
  id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB`); err != nil {
		t.Fatalf("create domain_event_outbox: %v", err)
	}
	if _, err := db.Exec("INSERT INTO domain_event_outbox (id) VALUES (1)"); err != nil {
		t.Fatalf("seed domain_event_outbox: %v", err)
	}
	insertMigrationUser(t, db, 1, "", false)
}

func insertMigrationUser(t *testing.T, db *sql.DB, id int, phone string, deleted bool) {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = "2026-01-01 00:00:00.000"
	}
	if _, err := db.Exec(
		"INSERT INTO `users` (`id`, `phone`, `deleted_at`) VALUES (?, ?, ?)",
		id,
		phone,
		deletedAt,
	); err != nil {
		t.Fatalf("insert user %d: %v", id, err)
	}
}

func migrationUserRows(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`
SELECT id, phone, IF(deleted_at IS NULL, 'active', 'deleted')
FROM users ORDER BY id`)
	if err != nil {
		t.Fatalf("query users: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var result string
	for rows.Next() {
		var id int
		var phone, status string
		if err := rows.Scan(&id, &phone, &status); err != nil {
			t.Fatalf("scan user: %v", err)
		}
		result += fmt.Sprintf("%d:%s:%s;", id, phone, status)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate users: %v", err)
	}
	return result
}

func assertActivePhoneGuardSchema(t *testing.T, db *sql.DB, want bool) {
	t.Helper()
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))
	var columns, indexes int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'users' AND COLUMN_NAME = 'active_phone'`, database).Scan(&columns); err != nil {
		t.Fatalf("query active_phone: %v", err)
	}
	if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'users' AND INDEX_NAME = 'uk_users_active_phone'`, database).Scan(&indexes); err != nil {
		t.Fatalf("query active-phone index: %v", err)
	}
	wantCount := 0
	if want {
		wantCount = 1
	}
	if columns != wantCount || indexes != wantCount {
		t.Fatalf("active-phone schema column/index = %d/%d, want %d/%d", columns, indexes, wantCount, wantCount)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, table string, want bool) {
	t.Helper()
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))
	var count int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, database, table).Scan(&count); err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}
	if (count == 1) != want {
		t.Fatalf("table %s exists=%v, want %v", table, count == 1, want)
	}
}

func assertTableRowCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM `" + table + "`").Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s rows = %d, want %d", table, count, want)
	}
}
