package visibility

import (
	"context"
	"testing"
	"time"

	domainvisibility "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

type countingVisibility struct {
	calls int
	ids   []int64
}

func (c *countingVisibility) VisibleProfileIDs(context.Context, domainvisibility.Principal) ([]int64, error) {
	c.calls++
	return append([]int64(nil), c.ids...), nil
}

func TestCachedReaderHitsCache(t *testing.T) {
	inner := &countingVisibility{ids: []int64{1, 2}}
	cached := NewCachedReader(inner, time.Minute).(*CachedReader)
	cached.now = func() time.Time { return time.Unix(100, 0) }

	principal := domainvisibility.Principal{OperatorID: 42}
	ctx := context.Background()

	ids1, err := cached.VisibleProfileIDs(ctx, principal)
	if err != nil || len(ids1) != 2 {
		t.Fatalf("first ids = %v err = %v", ids1, err)
	}
	ids2, err := cached.VisibleProfileIDs(ctx, principal)
	if err != nil || len(ids2) != 2 {
		t.Fatalf("second ids = %v err = %v", ids2, err)
	}
	if inner.calls != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.calls)
	}
}

func TestCachedReaderZeroTTLReturnsInner(t *testing.T) {
	inner := &countingVisibility{ids: []int64{1}}
	out := NewCachedReader(inner, 0)
	if out != inner {
		t.Fatal("zero ttl should return inner unchanged")
	}
}

func TestCachedReaderDefensiveCopy(t *testing.T) {
	inner := &countingVisibility{ids: []int64{1, 2}}
	cached := NewCachedReader(inner, time.Minute).(*CachedReader)
	cached.now = func() time.Time { return time.Unix(100, 0) }

	ctx := context.Background()
	principal := domainvisibility.Principal{OperatorID: 42}
	ids1, _ := cached.VisibleProfileIDs(ctx, principal)
	ids1[0] = 99
	ids2, _ := cached.VisibleProfileIDs(ctx, principal)
	if ids2[0] != 1 {
		t.Fatalf("cache returned mutated slice: %v", ids2)
	}
}
