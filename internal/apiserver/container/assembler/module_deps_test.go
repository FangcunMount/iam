package assembler

import (
	"testing"

	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
)

func TestSuggestModuleInitializeWithDepsDisabledDoesNotRequireDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: appsuggest.Config{Enable: false}}); err != nil {
		t.Fatalf("InitializeWithDeps() error = %v", err)
	}
	if module.Service != nil {
		t.Fatalf("Service = %#v, want nil when suggest is disabled", module.Service)
	}
}

func TestSuggestModuleInitializeWithDepsEnabledRequiresDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: appsuggest.Config{Enable: true}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestUserModuleInitializeWithDepsRequiresDB(t *testing.T) {
	module := NewUserModule()

	if err := module.InitializeWithDeps(UserModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestIDPModuleInitializeWithDepsRequiresDependencies(t *testing.T) {
	module := NewIDPModule()

	if err := module.InitializeWithDeps(IDPModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestAuthzModuleInitializeWithDepsRequiresDB(t *testing.T) {
	module := NewAuthzModule()

	if err := module.InitializeWithDeps(AuthzModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}
