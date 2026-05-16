package suggest

import (
	"context"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProfileVisibilityResolverVisibleProfileIDsByCreatedBy(t *testing.T) {
	db := newTestProfilesDB(t)
	now := time.Now()
	if err := db.Exec(`INSERT INTO profiles (id, name, created_by, created_at, updated_at, version) VALUES (1, 'a', 100, ?, ?, 1)`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO profiles (id, name, created_by, created_at, updated_at, version) VALUES (2, 'b', 200, ?, ?, 1)`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO profiles (id, name, created_by, created_at, updated_at, deleted_at, version) VALUES (3, 'c', 100, ?, ?, ?, 1)`, now, now, now).Error; err != nil {
		t.Fatal(err)
	}

	r := NewProfileVisibilityResolver(db)
	ids, err := r.VisibleProfileIDs(context.Background(), domainsuggest.OperatingPrincipal{OperatorID: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("ids = %v, want [1]", ids)
	}
}

func TestProfileVisibilityResolverNilDBReturnsEmpty(t *testing.T) {
	r := NewProfileVisibilityResolver(nil)
	ids, err := r.VisibleProfileIDs(context.Background(), domainsuggest.OperatingPrincipal{OperatorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if ids != nil {
		t.Fatalf("ids = %v, want nil", ids)
	}
}

func newTestProfilesDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
CREATE TABLE profiles (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  created_by INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME,
  version INTEGER NOT NULL DEFAULT 1
)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
