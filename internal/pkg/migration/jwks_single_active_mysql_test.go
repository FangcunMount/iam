package migration

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestJWKSSingleActiveMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000016_jwks_single_active_guard.up.sql")
	down := migrationSQL(t, "000016_jwks_single_active_guard.down.sql")

	t.Run("zero active supports up down up", func(t *testing.T) {
		resetLegacyJWKSTable(t, db)
		insertMigrationKey(t, db, "grace", 2)
		insertMigrationKey(t, db, "retired", 3)

		mustExecMigrationSQL(t, db, up)
		assertJWKSGuardSchema(t, db, true)
		insertMigrationKey(t, db, "grace-2", 2)
		insertMigrationKey(t, db, "retired-2", 3)
		insertMigrationKey(t, db, "active", 1)
		if _, err := db.Exec("INSERT INTO `jwks_keys` (`kid`, `status`) VALUES (?, ?)", "second-active", 1); err == nil {
			t.Fatal("single-active guard accepted a second active key")
		}

		rowsBefore := migrationKeyRows(t, db)
		mustExecMigrationSQL(t, db, down)
		assertJWKSGuardSchema(t, db, false)
		if got := migrationKeyRows(t, db); got != rowsBefore {
			t.Fatalf("down migration changed key rows: got %q want %q", got, rowsBefore)
		}
		mustExecMigrationSQL(t, db, up)
		assertJWKSGuardSchema(t, db, true)
	})

	t.Run("one active preserves data", func(t *testing.T) {
		resetLegacyJWKSTable(t, db)
		insertMigrationKey(t, db, "active", 1)
		insertMigrationKey(t, db, "grace", 2)
		rowsBefore := migrationKeyRows(t, db)

		mustExecMigrationSQL(t, db, up)
		assertJWKSGuardSchema(t, db, true)
		if got := migrationKeyRows(t, db); got != rowsBefore {
			t.Fatalf("up migration changed key rows: got %q want %q", got, rowsBefore)
		}
	})

	t.Run("multiple active fails atomically", func(t *testing.T) {
		resetLegacyJWKSTable(t, db)
		insertMigrationKey(t, db, "active-a", 1)
		insertMigrationKey(t, db, "active-b", 1)
		insertMigrationKey(t, db, "grace", 2)
		rowsBefore := migrationKeyRows(t, db)

		if _, err := db.Exec(up); err == nil {
			t.Fatal("up migration succeeded with multiple active keys")
		}
		if got := migrationKeyRows(t, db); got != rowsBefore {
			t.Fatalf("failed migration changed key rows: got %q want %q", got, rowsBefore)
		}
		assertJWKSGuardSchema(t, db, false)
	})
}

func openMigrationMySQL(t *testing.T) *sql.DB {
	t.Helper()
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		t.Skip("MYSQL_HOST is required for MySQL migration semantics")
	}
	port, err := strconv.Atoi(migrationEnvOr("MYSQL_PORT", "3306"))
	if err != nil {
		t.Fatalf("parse MYSQL_PORT: %v", err)
	}
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC&multiStatements=true",
		migrationEnvOr("MYSQL_USER", migrationEnvOr("MYSQL_USERNAME", "iam")),
		os.Getenv("MYSQL_PASSWORD"),
		host,
		port,
		database,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping MySQL: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS `jwks_keys`")
		_ = db.Close()
	})
	return db
}

func resetLegacyJWKSTable(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DROP TABLE IF EXISTS `jwks_keys`"); err != nil {
		t.Fatalf("drop jwks_keys: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE ` + "`jwks_keys`" + ` (
  ` + "`id`" + ` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  ` + "`kid`" + ` VARCHAR(64) NOT NULL,
  ` + "`status`" + ` TINYINT NOT NULL,
  PRIMARY KEY (` + "`id`" + `),
  UNIQUE KEY ` + "`uk_jwks_keys_kid`" + ` (` + "`kid`" + `)
) ENGINE=InnoDB`); err != nil {
		t.Fatalf("create legacy jwks_keys: %v", err)
	}
}

func insertMigrationKey(t *testing.T, db *sql.DB, kid string, status int) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO `jwks_keys` (`kid`, `status`) VALUES (?, ?)", kid, status); err != nil {
		t.Fatalf("insert key %s: %v", kid, err)
	}
}

func mustExecMigrationSQL(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	if _, err := db.Exec(statement); err != nil {
		t.Fatalf("execute migration SQL: %v", err)
	}
}

func assertJWKSGuardSchema(t *testing.T, db *sql.DB, want bool) {
	t.Helper()
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))
	var columns int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'jwks_keys' AND COLUMN_NAME = 'active_guard'`, database).Scan(&columns); err != nil {
		t.Fatalf("query active_guard: %v", err)
	}
	var indexes int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'jwks_keys' AND INDEX_NAME = 'uk_jwks_keys_single_active'`, database).Scan(&indexes); err != nil {
		t.Fatalf("query single-active index: %v", err)
	}
	wantCount := 0
	if want {
		wantCount = 1
	}
	if columns != wantCount || indexes != wantCount {
		t.Fatalf("guard schema column/index = %d/%d, want %d/%d", columns, indexes, wantCount, wantCount)
	}
}

func migrationKeyRows(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query("SELECT `kid`, `status` FROM `jwks_keys` ORDER BY `kid`")
	if err != nil {
		t.Fatalf("query key rows: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var result string
	for rows.Next() {
		var kid string
		var status int
		if err := rows.Scan(&kid, &status); err != nil {
			t.Fatalf("scan key row: %v", err)
		}
		result += fmt.Sprintf("%s:%d;", kid, status)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate key rows: %v", err)
	}
	return result
}

func migrationEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
