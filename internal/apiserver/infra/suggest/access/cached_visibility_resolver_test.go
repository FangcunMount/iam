package access

import (
	"context"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

type countingVisibility struct {
	calls int
	ids   []int64
}

func (c *countingVisibility) VisibleProfileIDs(context.Context, domainsuggest.OperatingPrincipal) ([]int64, error) {
	c.calls++
	return append([]int64(nil), c.ids...), nil
}

func TestCachedProfileVisibilityResolverHitsCache(t *testing.T) {
	inner := &countingVisibility{ids: []int64{1, 2}}
	cached := NewCachedProfileVisibilityResolver(inner, time.Minute).(*CachedProfileVisibilityResolver)
	cached.now = func() time.Time { return time.Unix(100, 0) }

	principal := domainsuggest.OperatingPrincipal{OperatorID: 42}
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

func TestCachedProfileVisibilityResolverZeroTTLReturnsInner(t *testing.T) {
	inner := &countingVisibility{ids: []int64{1}}
	out := NewCachedProfileVisibilityResolver(inner, 0)
	if out != inner {
		t.Fatal("zero ttl should return inner unchanged")
	}
}
