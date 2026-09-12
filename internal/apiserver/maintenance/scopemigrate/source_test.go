package scopemigrate

import (
	"context"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestBusinessSourceReadsMembershipAndStores(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	}()
	sqlDB.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE operators (id INTEGER, org_id INTEGER, user_id INTEGER, is_active BOOLEAN, deleted_at DATETIME)",
		"CREATE TABLE actor_stores (id INTEGER, org_id INTEGER, is_active BOOLEAN)",
		"INSERT INTO operators VALUES (4,7,3,TRUE,NULL),(5,7,6,FALSE,NULL),(6,7,9,TRUE,'2026-01-01')",
		"INSERT INTO actor_stores VALUES (8,7,TRUE)",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	facts, err := LoadBusinessFacts(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Operators) != 2 || facts.Operators[0].UserID != "3" || facts.Operators[1].Active || len(facts.Stores) != 1 || facts.Stores[0].OrgID != "7" {
		t.Fatalf("incorrect source mapping %+v", facts)
	}
	// A failed source query is an error, never an empty valid migration population.
	if err := db.Exec("DROP TABLE actor_stores").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBusinessFacts(context.Background(), db); err == nil {
		t.Fatal("missing source accepted")
	}
}
