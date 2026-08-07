package migration

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRetireUnusedPlatformTablesMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000021_retire_unused_platform_tables.up.sql")
	down := migrationSQL(t, "000021_retire_unused_platform_tables.down.sql")
	t.Cleanup(func() { resetPlatformRetirementFixture(t, db) })

	t.Run("drops both tables and tolerates absent tables", func(t *testing.T) {
		resetPlatformRetirementFixture(t, db)
		createPlatformRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, up)
		assertTableExists(t, db, "tenants", false)
		assertTableExists(t, db, "data_dictionary", false)
		mustExecMigrationSQL(t, db, up)
	})

	t.Run("dependency aborts before either table is removed", func(t *testing.T) {
		resetPlatformRetirementFixture(t, db)
		createPlatformRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "CREATE VIEW legacy_tenant_view AS SELECT id FROM tenants")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "platform table database dependencies still exist") {
			t.Fatalf("migration error = %v, want dependency failure", err)
		}
		assertTableExists(t, db, "tenants", true)
		assertTableExists(t, db, "data_dictionary", true)
	})

	t.Run("down fails closed", func(t *testing.T) {
		if _, err := db.Exec(down); err == nil || !strings.Contains(err.Error(), "irreversible") {
			t.Fatalf("down migration error = %v, want irreversible failure", err)
		}
	})
}

func resetPlatformRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP VIEW IF EXISTS legacy_tenant_view",
		"DROP TABLE IF EXISTS data_dictionary",
		"DROP TABLE IF EXISTS tenants",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset platform retirement fixture with %q: %v", statement, err)
		}
	}
}

func createPlatformRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE tenants (
  id VARCHAR(64) NOT NULL PRIMARY KEY,
  name VARCHAR(100) NOT NULL
); CREATE TABLE data_dictionary (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  dict_type VARCHAR(50) NOT NULL,
  dict_code VARCHAR(50) NOT NULL
); INSERT INTO tenants (id, name) VALUES ('legacy', 'legacy');
INSERT INTO data_dictionary (dict_type, dict_code) VALUES ('legacy', 'legacy')`); err != nil {
		t.Fatalf("create platform retirement fixture: %v", err)
	}
}
