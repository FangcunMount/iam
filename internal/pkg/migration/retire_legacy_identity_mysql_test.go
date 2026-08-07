package migration

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRetiredIdentityTablesMigrationMySQL(t *testing.T) {
	db := openMigrationMySQL(t)
	up := migrationSQL(t, "000019_retire_legacy_tables.up.sql")
	down := migrationSQL(t, "000019_retire_legacy_tables.down.sql")
	t.Cleanup(func() { resetRetiredIdentityFixture(t, db) })

	t.Run("backfills canonical facts before dropping legacy tables", func(t *testing.T) {
		resetRetiredIdentityFixture(t, db)
		createLegacyIdentityFixture(t, db)
		seedLegacyIdentityFixture(t, db)

		mustExecMigrationSQL(t, db, up)

		assertTableExists(t, db, "children", false)
		assertTableExists(t, db, "guardianships", false)
		assertTableRowCount(t, db, "profiles", 2)
		assertTableRowCount(t, db, "profile_links", 2)

		var typ, relation string
		if err := db.QueryRow("SELECT type, relation FROM profile_links WHERE id = 21").Scan(&typ, &relation); err != nil {
			t.Fatalf("query migrated profile link: %v", err)
		}
		if typ != "relation" || relation != "parent" {
			t.Fatalf("migrated profile link type/relation = %s/%s, want relation/parent", typ, relation)
		}
	})

	t.Run("canonical conflict aborts without overwriting current data", func(t *testing.T) {
		resetRetiredIdentityFixture(t, db)
		mustExecMigrationSQL(t, db, up)
		if _, err := db.Exec("INSERT INTO profiles (id, name) VALUES (10, 'current-name')"); err != nil {
			t.Fatalf("seed current profile: %v", err)
		}
		createLegacyIdentityFixture(t, db)
		seedLegacyIdentityFixture(t, db)

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "legacy Identity parity is incomplete") {
			t.Fatalf("migration error = %v, want Identity parity failure", err)
		}

		var name string
		if err := db.QueryRow("SELECT name FROM profiles WHERE id = 10").Scan(&name); err != nil {
			t.Fatalf("query current profile: %v", err)
		}
		if name != "current-name" {
			t.Fatalf("migration overwrote current profile name: got %q", name)
		}
		assertTableExists(t, db, "children", true)
		assertTableExists(t, db, "guardianships", true)
	})

	t.Run("database dependency aborts before drop", func(t *testing.T) {
		resetRetiredIdentityFixture(t, db)
		createLegacyIdentityFixture(t, db)
		seedLegacyIdentityFixture(t, db)
		if _, err := db.Exec("CREATE VIEW legacy_children_view AS SELECT id FROM children"); err != nil {
			t.Fatalf("create legacy dependency view: %v", err)
		}

		if _, err := db.Exec(up); err == nil || !strings.Contains(err.Error(), "legacy table database dependencies still exist") {
			t.Fatalf("migration error = %v, want dependency failure", err)
		}
		assertTableExists(t, db, "children", true)
		assertTableExists(t, db, "guardianships", true)
	})

	t.Run("down fails closed", func(t *testing.T) {
		if _, err := db.Exec(down); err == nil || !strings.Contains(err.Error(), "irreversible") {
			t.Fatalf("down migration error = %v, want irreversible failure", err)
		}
	})
}

func resetRetiredIdentityFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP VIEW IF EXISTS legacy_children_view",
		"DROP TABLE IF EXISTS guardianships",
		"DROP TABLE IF EXISTS children",
		"DROP TABLE IF EXISTS profile_links",
		"DROP TABLE IF EXISTS profiles",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset retired Identity fixture with %q: %v", statement, err)
		}
	}
}

func createLegacyIdentityFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
CREATE TABLE children (
  id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  id_card VARCHAR(20) NULL,
  gender TINYINT NOT NULL DEFAULT 0,
  birthday VARCHAR(10) NULL,
  height BIGINT NULL,
  weight BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  version INT UNSIGNED NOT NULL DEFAULT 1,
  UNIQUE KEY uk_id_card (id_card)
) ENGINE=InnoDB;
CREATE TABLE guardianships (
  id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  child_id BIGINT UNSIGNED NOT NULL,
  relation VARCHAR(16) NOT NULL,
  established_at DATETIME NULL,
  revoked_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  version INT UNSIGNED NOT NULL DEFAULT 1
) ENGINE=InnoDB`); err != nil {
		t.Fatalf("create legacy Identity fixture: %v", err)
	}
}

func seedLegacyIdentityFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO children (id, name, id_card, gender) VALUES
  (10, 'legacy-a', 'id-card-10', 1),
  (11, 'legacy-b', 'id-card-11', 2);
INSERT INTO guardianships (id, user_id, child_id, relation) VALUES
  (20, 100, 10, 'self'),
  (21, 100, 11, 'parent')`); err != nil {
		t.Fatalf("seed legacy Identity fixture: %v", err)
	}
}
