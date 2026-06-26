package suggest

import (
	"testing"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
)

func TestSuggestModuleInitializeWithDepsDisabledDoesNotRequireDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: appsuggest.Config{Enable: false}}); err != nil {
		t.Fatalf("InitializeWithDeps() error = %v", err)
	}
	if module.IsInitialized() {
		t.Fatalf("IsInitialized() = true, want false when suggest is disabled")
	}
}

func TestSuggestModuleInitializeWithDepsEnabledRequiresDB(t *testing.T) {
	module := NewSuggestModule()

	if err := module.InitializeWithDeps(SuggestModuleDeps{Config: appsuggest.Config{Enable: true}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}
