package suggest

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

func TestVisibilityReaderReturnsActiveProfilesByCreator(t *testing.T) {
	db := newSuggestLoaderSQLiteDB(t)
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO profiles (id, name, created_by, created_at, updated_at, version)
		 VALUES (1, 'active', 42, ?, ?, 1)`, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		`INSERT INTO profiles (id, name, created_by, created_at, updated_at, deleted_at, version)
		 VALUES (2, 'deleted', 42, ?, ?, ?, 1)`, now, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		`INSERT INTO profiles (id, name, created_by, created_at, updated_at, version)
		 VALUES (3, 'other', 7, ?, ?, 1)`, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}

	ids, err := NewVisibilityReader(db).VisibleProfileIDs(
		context.Background(),
		visibility.Principal{OperatorID: 42},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("ids = %v, want [1]", ids)
	}
}

func TestVisibilityReaderInvalidPrincipalReturnsEmpty(t *testing.T) {
	ids, err := NewVisibilityReader(nil).VisibleProfileIDs(
		context.Background(),
		visibility.Principal{},
	)
	if err != nil || ids != nil {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
}

func TestVisibilityReaderPropagatesQueryError(t *testing.T) {
	db := newSuggestLoaderSQLiteDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = NewVisibilityReader(db).VisibleProfileIDs(
		context.Background(),
		visibility.Principal{OperatorID: 42},
	)
	if err == nil {
		t.Fatal("VisibleProfileIDs() error = nil")
	}
}
