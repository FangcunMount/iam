package ratelimit

import (
	"testing"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
)

func TestMemoryLimiterEvictsWhenMapFull(t *testing.T) {
	lim := NewMemoryLimiter(appsuggest.RateLimitConfig{
		PerOperatorQPS:        100,
		PerOperatorBurst:      100,
		OperatorMapMaxEntries: 2,
	})

	if !lim.Allow(1, false) || !lim.Allow(2, false) {
		t.Fatal("expected first operators allowed")
	}
	if !lim.Allow(3, false) {
		t.Fatal("expected third operator allowed after eviction")
	}
}
