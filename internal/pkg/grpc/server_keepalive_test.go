package grpc

import (
	"testing"
	"time"
)

func TestIAMKeepaliveEnforcementPolicy(t *testing.T) {
	t.Parallel()

	policy := iamKeepaliveEnforcementPolicy()
	if policy.MinTime != 5*time.Minute {
		t.Fatalf("MinTime = %s, want 5m", policy.MinTime)
	}
	if policy.PermitWithoutStream {
		t.Fatal("PermitWithoutStream = true, want false")
	}
}
