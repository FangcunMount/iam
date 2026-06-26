package identity

import (
	"testing"

	"gorm.io/gorm"
)

func TestIdentityModuleInitializeWithDepsRequiresDB(t *testing.T) {
	module := NewIdentityModule()

	if err := module.InitializeWithDeps(IdentityModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestIdentityModuleInitializeWithDepsAcceptsDB(t *testing.T) {
	module := NewIdentityModule()
	if err := module.InitializeWithDeps(IdentityModuleDeps{DB: &gorm.DB{}}); err != nil {
		t.Fatalf("InitializeWithDeps() error = %v", err)
	}
}
