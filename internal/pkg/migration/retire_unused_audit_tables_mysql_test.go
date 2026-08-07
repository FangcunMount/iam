package migration

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRetireUnusedAuditTablesMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000022_retire_unused_audit_tables.up.sql")
	down := migrationSQL(t, "000022_retire_unused_audit_tables.down.sql")
	t.Cleanup(func() { resetAuditRetirementFixture(t, db) })

	t.Run("drops all tables and tolerates absent tables", func(t *testing.T) {
		resetAuditRetirementFixture(t, db)
		createAuditRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, up)
		for _, table := range []string{"operation_logs", "audit_logs", "auth_token_audit"} {
			assertTableExists(t, db, table, false)
		}
		mustExecMigrationSQL(t, db, up)
	})

	t.Run("dependency aborts before any table is removed", func(t *testing.T) {
		resetAuditRetirementFixture(t, db)
		createAuditRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "CREATE VIEW legacy_audit_view AS SELECT id FROM audit_logs")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "audit table database dependencies still exist") {
			t.Fatalf("migration error = %v, want dependency failure", err)
		}
		for _, table := range []string{"operation_logs", "audit_logs", "auth_token_audit"} {
			assertTableExists(t, db, table, true)
		}
	})

	t.Run("down fails closed", func(t *testing.T) {
		if _, err := db.Exec(down); err == nil || !strings.Contains(err.Error(), "irreversible") {
			t.Fatalf("down migration error = %v, want irreversible failure", err)
		}
	})
}

func resetAuditRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP VIEW IF EXISTS legacy_audit_view",
		"DROP TABLE IF EXISTS auth_token_audit",
		"DROP TABLE IF EXISTS audit_logs",
		"DROP TABLE IF EXISTS operation_logs",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset audit retirement fixture with %q: %v", statement, err)
		}
	}
}

func createAuditRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExecMigrationSQL(t, db, `CREATE TABLE operation_logs (id BIGINT NOT NULL PRIMARY KEY);
CREATE TABLE audit_logs (id BIGINT NOT NULL PRIMARY KEY);
CREATE TABLE auth_token_audit (id BIGINT NOT NULL PRIMARY KEY)`)
}
