package migration

import (
	"database/sql"
	"testing"
)

func TestAuthZSeedAssignmentRepairMigrationMySQL(t *testing.T) {
	migrationDB := openMigrationMySQL(t)
	database := migrationEnvOr("MYSQL_DATABASE", migrationEnvOr("MYSQL_DBNAME", "iam_test"))

	var existingTables int
	if err := migrationDB.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, database).Scan(&existingTables); err != nil {
		t.Fatalf("count existing repair fixture tables: %v", err)
	}
	if existingTables != 0 {
		t.Fatalf("seed-assignment repair test requires an empty dedicated database, found %d tables", existingTables)
	}

	migrator := NewMigrator(migrationDB, &Config{Enabled: true, Database: database})
	version, migrated, err := migrator.RunTo(25)
	if err != nil {
		t.Fatalf("migrate repair fixture to version 25: %v", err)
	}
	if !migrated || version != 25 {
		t.Fatalf("repair fixture migration result = version %d migrated=%v, want version 25 migrated=true", version, migrated)
	}

	db := openMigrationMySQL(t)
	seedRebuiltRolesAndAssignments(t, db)

	migrator = NewMigrator(db, &Config{Enabled: true, Database: database})
	version, migrated, err = migrator.RunTo(26)
	if err != nil {
		t.Fatalf("migrate repair fixture to version 26: %v", err)
	}
	if !migrated || version != 26 {
		t.Fatalf("repair migration result = version %d migrated=%v, want version 26 migrated=true", version, migrated)
	}

	db = openMigrationMySQL(t)
	assertAssignmentCount(t, db, `
SELECT COUNT(*) FROM authz_assignments
WHERE id IN (902000001, 902000002, 902000003, 902000004, 902000005, 902000006)
  AND deleted_at IS NOT NULL`, 6, "verified stale assignments retired")
	assertAssignmentCount(t, db, `
SELECT COUNT(*) FROM authz_assignments
WHERE id IN (613500091088515630, 613500091021406766, 613500090887189038,
             613486857019208238, 613486857069539886, 613500090954297902)
  AND deleted_at IS NULL`, 6, "replacement assignments preserved")
	assertAssignmentCount(t, db, `
SELECT COUNT(*) FROM authz_assignments
WHERE id = 902009999 AND deleted_at IS NULL`, 1, "unverified orphan assignment preserved for preflight")
}

func seedRebuiltRolesAndAssignments(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
DELETE FROM authz_roles
WHERE id IN (900000001, 2, 900000101, 900000102);

INSERT INTO authz_roles
    (id, name, display_name, tenant_id, is_system, description, created_at, updated_at)
VALUES
    (613485615136125486, 'super_admin', '平台超级管理员', 'platform', 1, 'replacement', UTC_TIMESTAMP(), UTC_TIMESTAMP()),
    (613485615152902702, 'tenant_admin', '租户管理员', 'fangcun', 1, 'replacement', UTC_TIMESTAMP(), UTC_TIMESTAMP()),
    (613485615186457134, 'qs:admin', 'QS管理员', 'fangcun', 1, 'replacement', UTC_TIMESTAMP(), UTC_TIMESTAMP()),
    (613485615203234350, 'qs:content_manager', '内容管理员', 'fangcun', 1, 'replacement', UTC_TIMESTAMP(), UTC_TIMESTAMP());

INSERT INTO authz_assignments
    (id, subject_type, subject_id, role_id, tenant_id, granted_by, granted_at)
VALUES
    (613500091088515630, 'user', '10001', 613485615136125486, 'platform', 'fixture', UTC_TIMESTAMP()),
    (613500091021406766, 'user', '10001', 613485615152902702, 'fangcun', 'fixture', UTC_TIMESTAMP()),
    (613500090887189038, 'user', '10001', 613485615186457134, 'fangcun', 'fixture', UTC_TIMESTAMP()),
    (613486857019208238, 'user', '110001', 613485615152902702, 'fangcun', 'fixture', UTC_TIMESTAMP()),
    (613486857069539886, 'user', '110001', 613485615186457134, 'fangcun', 'fixture', UTC_TIMESTAMP()),
    (613500090954297902, 'user', '110002', 613485615203234350, 'fangcun', 'fixture', UTC_TIMESTAMP()),
    (902009999, 'user', 'unverified-subject', 999999999, 'fangcun', 'fixture', UTC_TIMESTAMP());
`); err != nil {
		t.Fatalf("seed rebuilt roles and assignments: %v", err)
	}
}

func assertAssignmentCount(t *testing.T, db *sql.DB, query string, want int, label string) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("query %s: %v", label, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", label, got, want)
	}
}
