package suggest

import (
	"testing"
)

func TestSuggestModuleInitializeWithDepsDisabledDoesNotRequireDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: ModuleConfig{Enable: false}}); err != nil {
		t.Fatalf("InitializeWithDeps() error = %v", err)
	}
	if module.IsInitialized() {
		t.Fatalf("IsInitialized() = true, want false when suggest is disabled")
	}
}

func TestSuggestModuleInitializeWithDepsEnabledRequiresDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: ModuleConfig{Enable: true}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}
