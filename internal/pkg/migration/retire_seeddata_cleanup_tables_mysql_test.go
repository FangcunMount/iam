package migration

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

const (
	cleanupProfilesTable      = "cbpt_profiles_s812v2"
	cleanupProfileLinksTable  = "cbpt_profile_links_s812v2"
	cleanupProfilesBackup     = "cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1"
	cleanupProfileLinksBackup = "cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1"
	cleanupRetirementRowCount = 1359
)

func TestRetireSeeddataCleanupTablesMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000024_retire_seeddata_cleanup_tables.up.sql")
	down := migrationSQL(t, "000024_retire_seeddata_cleanup_tables.down.sql")
	t.Cleanup(func() { resetSeeddataCleanupRetirementFixture(t, db) })

	t.Run("drops the verified duplicate copies and tolerates already absent tables", func(t *testing.T) {
		resetSeeddataCleanupRetirementFixture(t, db)
		createSeeddataCleanupCanonicalFixture(t, db)
		createSeeddataCleanupRetirementFixture(t, db)
		seedSeeddataCleanupRetirementFixture(t, db)

		mustExecMigrationSQL(t, db, up)
		for _, table := range seeddataCleanupTables() {
			assertTableExists(t, db, table, false)
		}
		assertTableRowCount(t, db, "profiles", 0)
		assertTableRowCount(t, db, "profile_links", 0)
		mustExecMigrationSQL(t, db, up)
	})

	t.Run("partial table set aborts", func(t *testing.T) {
		resetSeeddataCleanupRetirementFixture(t, db)
		createSeeddataCleanupCanonicalFixture(t, db)
		createSeeddataCleanupRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "DROP TABLE "+cleanupProfileLinksBackup)

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "table set or schema differs") {
			t.Fatalf("migration error = %v, want schema failure", err)
		}
		assertTableExists(t, db, cleanupProfilesTable, true)
		assertTableExists(t, db, cleanupProfilesBackup, true)
	})

	t.Run("content drift aborts before any table is removed", func(t *testing.T) {
		resetSeeddataCleanupRetirementFixture(t, db)
		createSeeddataCleanupCanonicalFixture(t, db)
		createSeeddataCleanupRetirementFixture(t, db)
		seedSeeddataCleanupRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "UPDATE "+cleanupProfilesBackup+" SET name = 'PROFILE-1' WHERE id = 100001")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "contents differ") {
			t.Fatalf("migration error = %v, want content failure", err)
		}
		for _, table := range seeddataCleanupTables() {
			assertTableExists(t, db, table, true)
		}
	})

	t.Run("canonical ID overlap aborts", func(t *testing.T) {
		resetSeeddataCleanupRetirementFixture(t, db)
		createSeeddataCleanupCanonicalFixture(t, db)
		createSeeddataCleanupRetirementFixture(t, db)
		seedSeeddataCleanupRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "INSERT INTO profiles (id) VALUES (100001)")

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "contents differ") {
			t.Fatalf("migration error = %v, want canonical overlap failure", err)
		}
		for _, table := range seeddataCleanupTables() {
			assertTableExists(t, db, table, true)
		}
	})

	t.Run("database dependency aborts", func(t *testing.T) {
		resetSeeddataCleanupRetirementFixture(t, db)
		createSeeddataCleanupCanonicalFixture(t, db)
		createSeeddataCleanupRetirementFixture(t, db)
		seedSeeddataCleanupRetirementFixture(t, db)
		mustExecMigrationSQL(t, db, "CREATE VIEW seeddata_cleanup_view AS SELECT id FROM "+cleanupProfilesTable)

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "database dependencies still exist") {
			t.Fatalf("migration error = %v, want dependency failure", err)
		}
		for _, table := range seeddataCleanupTables() {
			assertTableExists(t, db, table, true)
		}
	})

	t.Run("down fails closed", func(t *testing.T) {
		if _, err := db.Exec(down); err == nil || !strings.Contains(err.Error(), "irreversible") {
			t.Fatalf("down migration error = %v, want irreversible failure", err)
		}
	})
}

func seeddataCleanupTables() []string {
	return []string{
		cleanupProfilesTable,
		cleanupProfileLinksTable,
		cleanupProfilesBackup,
		cleanupProfileLinksBackup,
	}
}

func resetSeeddataCleanupRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP VIEW IF EXISTS seeddata_cleanup_view",
		"DROP TABLE IF EXISTS " + strings.Join(seeddataCleanupTables(), ", "),
		"DROP TABLE IF EXISTS profile_links",
		"DROP TABLE IF EXISTS profiles",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset seeddata cleanup retirement fixture with %q: %v", statement, err)
		}
	}
}

func createSeeddataCleanupCanonicalFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExecMigrationSQL(t, db, `CREATE TABLE profiles (id BIGINT UNSIGNED NOT NULL PRIMARY KEY) ENGINE=InnoDB;
CREATE TABLE profile_links (id BIGINT UNSIGNED NOT NULL PRIMARY KEY) ENGINE=InnoDB`)
}

func createSeeddataCleanupRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	profileSchema := `(id BIGINT UNSIGNED NOT NULL PRIMARY KEY, name VARCHAR(64) NOT NULL,
id_card VARCHAR(20) NULL, gender TINYINT NOT NULL DEFAULT 0, birthday VARCHAR(10) NULL,
created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME NULL,
created_by BIGINT UNSIGNED NOT NULL DEFAULT 0, updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0, version INT UNSIGNED NOT NULL DEFAULT 1) ENGINE=InnoDB`
	profileLinkSchema := `(id BIGINT UNSIGNED NOT NULL PRIMARY KEY, user_id BIGINT UNSIGNED NOT NULL,
profile_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL, relation VARCHAR(16) NOT NULL,
self_key BIGINT UNSIGNED NULL, established_at DATETIME NOT NULL, revoked_at DATETIME NULL,
created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME NULL,
created_by BIGINT UNSIGNED NOT NULL DEFAULT 0, updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0, version INT UNSIGNED NOT NULL DEFAULT 1) ENGINE=InnoDB`
	mustExecMigrationSQL(t, db, fmt.Sprintf(`CREATE TABLE %s %s;
CREATE TABLE %s %s;
CREATE TABLE %s %s;
CREATE TABLE %s %s`,
		cleanupProfilesTable, profileSchema,
		cleanupProfilesBackup, profileSchema,
		cleanupProfileLinksTable, profileLinkSchema,
		cleanupProfileLinksBackup, profileLinkSchema,
	))
}

func seedSeeddataCleanupRetirementFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin seeddata cleanup fixture: %v", err)
	}
	profileInsert, err := tx.Prepare("INSERT INTO " + cleanupProfilesTable + " (id, name, id_card, gender, birthday, created_at, updated_at, deleted_at, created_by, updated_by, deleted_by, version) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("prepare cleanup profiles: %v", err)
	}
	linkInsert, err := tx.Prepare("INSERT INTO " + cleanupProfileLinksTable + " (id, user_id, profile_id, type, relation, self_key, established_at, revoked_at, created_at, updated_at, deleted_at, created_by, updated_by, deleted_by, version) VALUES (?, ?, ?, ?, ?, NULL, ?, NULL, ?, ?, NULL, ?, ?, ?, ?)")
	if err != nil {
		_ = profileInsert.Close()
		_ = tx.Rollback()
		t.Fatalf("prepare cleanup profile links: %v", err)
	}
	for index := 1; index <= cleanupRetirementRowCount; index++ {
		id := 100000 + index
		if _, err := profileInsert.Exec(id, fmt.Sprintf("profile-%d", index), nil, index%3, nil, "2026-08-12 12:00:00", "2026-08-12 12:00:00", 1, 2, 0, 1); err != nil {
			_ = profileInsert.Close()
			_ = linkInsert.Close()
			_ = tx.Rollback()
			t.Fatalf("insert cleanup profile %d: %v", index, err)
		}
		if _, err := linkInsert.Exec(200000+index, 300000+index, id, "relation", "other", "2026-08-12 12:00:00", "2026-08-12 12:00:00", "2026-08-12 12:00:00", 1, 2, 0, 1); err != nil {
			_ = profileInsert.Close()
			_ = linkInsert.Close()
			_ = tx.Rollback()
			t.Fatalf("insert cleanup profile link %d: %v", index, err)
		}
	}
	_ = profileInsert.Close()
	_ = linkInsert.Close()
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seeddata cleanup fixture: %v", err)
	}
	mustExecMigrationSQL(t, db, "INSERT INTO "+cleanupProfilesBackup+" SELECT * FROM "+cleanupProfilesTable)
	mustExecMigrationSQL(t, db, "INSERT INTO "+cleanupProfileLinksBackup+" SELECT * FROM "+cleanupProfileLinksTable)
}
