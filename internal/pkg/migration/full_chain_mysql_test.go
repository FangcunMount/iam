package migration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFullMigrationChainAndBootstrapMySQL(t *testing.T) {
	migrationDB := openMigrationMySQL(t)
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))

	var existingTables int
	if err := migrationDB.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, database).Scan(&existingTables); err != nil {
		t.Fatalf("count existing migration test tables: %v", err)
	}
	if existingTables != 0 {
		t.Fatalf("full-chain migration test requires an empty dedicated database, found %d tables", existingTables)
	}

	version, migrated, err := NewMigrator(migrationDB, &Config{Enabled: true, Database: database}).Run()
	if err != nil {
		t.Fatalf("run full migration chain: %v", err)
	}
	if !migrated || version != 21 {
		t.Fatalf("full migration result = version %d migrated=%v, want version 21 migrated=true", version, migrated)
	}
	db := openMigrationMySQL(t)
	for _, retired := range []string{"children", "guardianships", "schema_version", "tenants", "data_dictionary"} {
		assertTableExists(t, db, retired, false)
	}
	for _, retained := range []string{"schema_migrations", "operation_logs", "audit_logs", "auth_token_audit"} {
		assertTableExists(t, db, retained, true)
	}

	_, currentFile, _, _ := runtime.Caller(0)
	bootstrapPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "configs", "mysql", "bootstrap.sql")
	bootstrap, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("read bootstrap SQL: %v", err)
	}
	for run := 1; run <= 2; run++ {
		if _, err := db.Exec(string(bootstrap)); err != nil {
			t.Fatalf("bootstrap run %d after current migration: %v", run, err)
		}
	}
	for _, retired := range []string{"tenants", "data_dictionary"} {
		assertTableExists(t, db, retired, false)
	}
}
