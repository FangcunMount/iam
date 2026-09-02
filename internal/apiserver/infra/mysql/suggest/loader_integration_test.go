package suggest

import (
	"context"
	"testing"
	"time"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
	suggestmemory "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/index/memory"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoaderFullDeltaEquivalenceOnActiveProfile(t *testing.T) {
	db := newSuggestLoaderSQLiteDB(t)
	now := time.Now().UTC()
	seedActiveProfile(db, t, 1, "张三", 100, "13800138000", now)

	loader := NewLoader(db, LoaderConfig{})
	fullProfiles, err := loader.Full(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(fullProfiles) != 1 || fullProfiles[0].DisplayName() != "张三" {
		t.Fatalf("full = %#v", fullProfiles)
	}

	store := suggestmemory.Load(fullProfiles, suggestmemory.Config{})
	candidates, err := store.Recall(context.Background(), appquery.RecallRequest{
		Keyword:         domainsearch.NewKeyword("张"),
		Intent:          domainsearch.IntentTextPrefix,
		CandidateBudget: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := domainsearch.SelectionPolicy{}.Select(candidates, visibility.NewScope(true, true, 0, nil, nil), domainsearch.NewKeyword("张"), 5)
	if len(out.Profiles) != 1 || out.Profiles[0].ID() != 1 {
		t.Fatalf("full suggest = %#v", out.Profiles)
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

	changes, err := loader.Delta(context.Background(), since)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Kind() != apprefresh.ChangeDelete {
		t.Fatalf("changes = %#v", changes)
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

	changes, err := loader.Delta(context.Background(), since)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Kind() != apprefresh.ChangeUpsert {
		t.Fatalf("changes = %#v", changes)
	}
	mobiles := changes[0].Profile().Mobiles()
	if len(mobiles) != 1 || mobiles[0] != "13900139000" {
		t.Fatalf("mobiles = %#v", mobiles)
	}
}

func TestProfilesToChanges(t *testing.T) {
	changes := profilesToChanges([]domainprofile.SuggestibleProfile{
		domainprofile.New(1, "a", nil, 1, 0, nil),
		domainprofile.New(2, "", nil, 1, 0, nil),
	})
	if len(changes) != 2 {
		t.Fatalf("len = %d", len(changes))
	}
	if changes[0].Kind() != apprefresh.ChangeUpsert || changes[1].Kind() != apprefresh.ChangeDelete {
		t.Fatalf("kinds = %d,%d", changes[0].Kind(), changes[1].Kind())
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
