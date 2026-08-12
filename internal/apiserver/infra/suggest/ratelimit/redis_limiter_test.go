package ratelimit

import (
	"testing"

	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisLimiterSharedQuotaAcrossInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	cfg := appsuggest.RateLimitConfig{
		PerOperatorQPS:   10,
		PerOperatorBurst: 2,
		Backend:          "redis",
	}
	lim1 := NewRedisLimiter(client, cfg)
	lim2 := NewRedisLimiter(client, cfg)

	if !lim1.Allow(100, false) || !lim1.Allow(100, false) {
		t.Fatal("expected first two requests allowed")
	}
	if lim1.Allow(100, false) {
		t.Fatal("third request on same limiter should be denied")
	}
	if lim2.Allow(100, false) {
		t.Fatal("third request on second limiter should share redis key and be denied")
	}
}

func TestRedisLimiterMobileBucketSeparate(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	lim := NewRedisLimiter(client, appsuggest.RateLimitConfig{
		PerOperatorQPS:                10,
		PerOperatorBurst:              1,
		MobileKeywordPerOperatorBurst: 1,
	})

	if !lim.Allow(7, false) {
		t.Fatal("std bucket should allow first")
	}
	if lim.Allow(7, false) {
		t.Fatal("std bucket should deny second")
	}
	if !lim.Allow(7, true) {
		t.Fatal("mobile bucket should allow first")
	}
}
