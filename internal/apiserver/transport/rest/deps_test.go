package rest

import "testing"

func TestModuleStatusIdentityAvailableUsesIdentityModuleKey(t *testing.T) {
	t.Parallel()

	status := ModuleStatus{
		Modules: map[string]ModuleState{
			moduleStateIdentity: {Bootstrapped: true, Available: true},
		},
	}
	if !status.identityAvailable() {
		t.Fatal("identityAvailable() = false, want true when identity module state is available")
	}

	status.Modules = map[string]ModuleState{
		"user module": {Bootstrapped: true, Available: true},
	}
	if status.identityAvailable() {
		t.Fatal("identityAvailable() = true, want false when only legacy user module key is present")
	}
}
