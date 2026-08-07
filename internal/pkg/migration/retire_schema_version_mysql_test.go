package migration

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRetireSchemaVersionMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000020_retire_schema_version.up.sql")
	down := migrationSQL(t, "000020_retire_schema_version.down.sql")
	t.Cleanup(func() { resetSchemaVersionRetirementFixture(t, db) })

	t.Run("drops the redundant table and tolerates an absent table", func(t *testing.T) {
		resetSchemaVersionRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, `CREATE TABLE schema_version (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  version VARCHAR(20) NOT NULL
); INSERT INTO schema_version (version) VALUES ('legacy')`)

		mustExecMigrationSQL(t, db, up)
		assertTableExists(t, db, "schema_version", false)
		mustExecMigrationSQL(t, db, up)
	})

	t.Run("dependency aborts before removal", func(t *testing.T) {
		resetSchemaVersionRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, `CREATE TABLE schema_version (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  version VARCHAR(20) NOT NULL
); CREATE VIEW legacy_schema_version_view AS SELECT id FROM schema_version`)

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "schema_version database dependencies still exist") {
			t.Fatalf("migration error = %v, want dependency failure", err)
		}
		assertTableExists(t, db, "schema_version", true)
	})

	t.Run("down fails closed", func(t *testing.T) {
		if _, err := db.Exec(down); err == nil || !strings.Contains(err.Error(), "irreversible") {
			t.Fatalf("down migration error = %v, want irreversible failure", err)
		}
	})
}

func resetSchemaVersionRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP VIEW IF EXISTS legacy_schema_version_view",
		"DROP TABLE IF EXISTS schema_version",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset schema_version retirement fixture with %q: %v", statement, err)
		}
	}
}
