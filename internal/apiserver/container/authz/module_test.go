package authz

import (
	"testing"

	"gorm.io/gorm"
)

func TestAuthzModuleInitializeWithDepsRequiresDB(t *testing.T) {
	module := NewAuthzModule()

	if err := module.InitializeWithDeps(AuthzModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestAuthzModuleInitializeWithDepsRequiresEventStager(t *testing.T) {
	module := NewAuthzModule()

	if err := module.InitializeWithDeps(AuthzModuleDeps{DB: &gorm.DB{}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing event stager error")
	}
}
