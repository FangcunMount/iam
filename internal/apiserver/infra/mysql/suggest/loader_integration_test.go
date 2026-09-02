package suggest

import (
	"context"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
	searchruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/search"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoaderFullDeltaEquivalenceOnActiveProfile(t *testing.T) {
	db := newSuggestLoaderSQLiteDB(t)
	now := time.Now().UTC()
	seedActiveProfile(db, t, 1, "张三", 100, "13800138000", now)

	loader := NewLoader(db, LoaderConfig{})
	fullTerms, err := loader.Full(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(fullTerms) != 1 || fullTerms[0].DisplayName != "张三" {
		t.Fatalf("full = %#v", fullTerms)
	}

	fullStore := searchruntime.Load(fullTerms)
	out := fullStore.SuggestProfile(domainsuggest.NewQueryWithMode("张", domainsuggest.SearchModePrefix, 5, 50, 8, 0), domainsuggest.ProfileAccessScope{AllProfile: true})
	if len(out) != 1 || out[0].ProfileID != 1 {
		t.Fatalf("full suggest = %#v", out)
	}
}

func TestLoaderDeltaDeleteOnProfileSoftDelete(t *testing.T) {
	db := newSuggestLoaderSQLiteDB(t)
	now := time.Now().UTC()
	seedActiveProfile(db, t, 1, "张三", 100, "13800138000", now)

	loader := NewLoader(db, LoaderConfig{})
	since := now.Add(-time.Hour)
	if _, err := loader.Full(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE profiles SET deleted_at = ?, updated_at = ? WHERE id = 1`, now, now).Error; err != nil {
		t.Fatal(err)
	}

	mutations, err := loader.Delta(context.Background(), since)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 1 || mutations[0].Operation != domainsuggest.ProfileIndexDelete {
		t.Fatalf("mutations = %#v", mutations)
	}
}

func TestLoaderDeltaUpsertAfterMobileChange(t *testing.T) {
	db := newSuggestLoaderSQLiteDB(t)
	now := time.Now().UTC()
	seedActiveProfile(db, t, 1, "张三", 100, "13800138000", now)

	loader := NewLoader(db, LoaderConfig{})
	since := now.Add(-time.Hour)
	if err := db.Exec(`UPDATE users SET phone = ?, updated_at = ? WHERE id = 1`, "13900139000", now).Error; err != nil {
		t.Fatal(err)
	}

	mutations, err := loader.Delta(context.Background(), since)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 1 || mutations[0].Operation != domainsuggest.ProfileIndexUpsert {
		t.Fatalf("mutations = %#v", mutations)
	}
	if len(mutations[0].Term.Mobiles) != 1 || mutations[0].Term.Mobiles[0] != "13900139000" {
		t.Fatalf("mobiles = %#v", mutations[0].Term.Mobiles)
	}
}

func TestTermsToMutations(t *testing.T) {
	mutations := termsToMutations([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "a", nil, 1, 0, nil),
		domainsuggest.NewProfileSearchTerm(2, "", nil, 1, 0, nil),
	})
	if len(mutations) != 2 {
		t.Fatalf("len = %d", len(mutations))
	}
	if mutations[0].Operation != domainsuggest.ProfileIndexUpsert || mutations[1].Operation != domainsuggest.ProfileIndexDelete {
		t.Fatalf("ops = %d,%d", mutations[0].Operation, mutations[1].Operation)
	}
}

func newSuggestLoaderSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE profiles (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  created_by INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME,
  version INTEGER NOT NULL DEFAULT 1
)`,
		`CREATE TABLE profile_links (
  profile_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  deleted_at DATETIME,
  revoked_at DATETIME,
  updated_at DATETIME NOT NULL
)`,
		`CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  phone TEXT,
  deleted_at DATETIME,
  updated_at DATETIME NOT NULL
)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func seedActiveProfile(db *gorm.DB, t *testing.T, profileID int64, name string, createdBy int64, phone string, now time.Time) {
	t.Helper()
	if err := db.Exec(`INSERT INTO profiles (id, name, created_by, created_at, updated_at, version) VALUES (?, ?, ?, ?, ?, 1)`,
		profileID, name, createdBy, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO users (id, phone, updated_at) VALUES (1, ?, ?)`, phone, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO profile_links (profile_id, user_id, updated_at) VALUES (?, 1, ?)`, profileID, now).Error; err != nil {
		t.Fatal(err)
	}
}
