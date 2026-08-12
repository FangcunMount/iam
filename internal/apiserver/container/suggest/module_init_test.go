package suggest

import (
	"strings"
	"testing"

	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSuggestModuleInitializeWithDepsProductionForbidsDisableMobileMask(t *testing.T) {
	module := NewSuggestModule()
	err := module.InitializeWithDeps(SuggestModuleDeps{
		DB:          &gorm.DB{},
		Environment: genericapiserver.EnvironmentProduction,
		Config: appsuggest.Config{
			Enable:            true,
			DisableMobileMask: true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "disable_mobile_mask") {
		t.Fatalf("InitializeWithDeps() error = %v, want production forbid disable_mobile_mask", err)
	}
}

func TestSuggestModuleInitializeWithDepsDevelopmentAllowsDisableMobileMask(t *testing.T) {
	module := NewSuggestModule()
	t.Cleanup(func() { _ = module.Cleanup() })

	err := module.InitializeWithDeps(SuggestModuleDeps{
		DB:          newSuggestInitSQLiteDB(t),
		Environment: genericapiserver.EnvironmentDevelopment,
		Config: appsuggest.Config{
			Enable:            true,
			DisableMobileMask: true,
			Required:          false,
		},
	})
	if err != nil {
		t.Fatalf("InitializeWithDeps() error = %v, want nil in development with DisableMobileMask", err)
	}
	if !module.IsInitialized() {
		t.Fatal("module not initialized")
	}
}

func newSuggestInitSQLiteDB(t *testing.T) *gorm.DB {
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
