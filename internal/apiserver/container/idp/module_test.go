package idp

import (
	"testing"
)

func TestModuleInitializeWithDepsRequiresDependencies(t *testing.T) {
	module := NewIDPModule()

	if err := module.InitializeWithDeps(IDPModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}
