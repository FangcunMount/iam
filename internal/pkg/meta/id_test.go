package meta

import (
	"testing"
)

func TestNewReturnsNonZeroPositiveID(t *testing.T) {
	t.Parallel()

	id := New()

	if id.IsZero() {
		t.Fatal("New() returned zero ID")
	}
	if id.Int64() <= 0 {
		t.Fatalf("New() = %d, want positive ID", id.Int64())
	}
}

func TestNewReturnsDifferentIDs(t *testing.T) {
	t.Parallel()

	first := New()
	second := New()

	if first == second {
		t.Fatalf("New() returned duplicate IDs: %s", first.String())
	}
}
