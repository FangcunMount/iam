package redis

import (
	"context"
	"testing"
	"time"

	challengeApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestOTPVerifierTryAcquire(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	verifier := NewOTPVerifier(client)
	ctx := context.Background()
	rawGateKey := otpSendGateRedisKey("+8613800138000", "login")

	ok, err := verifier.TryAcquire(ctx, "+8613800138000", "login", time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected first TryAcquire() to succeed")
	}
	if !mr.Exists(rawGateKey) {
		t.Fatalf("expected raw send gate key %q to exist", rawGateKey)
	}

	ok, err = verifier.TryAcquire(ctx, "+8613800138000", "login", time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire() second error = %v", err)
	}
	if ok {
		t.Fatalf("expected second TryAcquire() to fail during cooldown")
	}
}

func TestOTPVerifierTryConsumeQuota(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	verifier := NewOTPVerifier(client)
	ctx := context.Background()

	const limit = 2
	var lease challengeApp.OTPSendQuotaLease
	for i := 1; i <= limit; i++ {
		gotLease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", limit, time.Hour)
		if err != nil {
			t.Fatalf("TryConsume() #%d error = %v", i, err)
		}
		if !ok {
			t.Fatalf("expected TryConsume() #%d to be allowed within limit", i)
		}
		if gotLease.Member == "" {
			t.Fatalf("expected TryConsume() #%d to return a rollback lease", i)
		}
		lease = gotLease
	}

	_, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", limit, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() over-limit error = %v", err)
	}
	if ok {
		t.Fatalf("expected TryConsume() to be rejected when exceeding limit")
	}

	// 回退一次后，应重新允许一次发送。
	if err := verifier.Rollback(ctx, lease); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	_, ok, err = verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", limit, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() after rollback error = %v", err)
	}
	if !ok {
		t.Fatalf("expected TryConsume() to be allowed again after rollback")
	}
}

func TestOTPVerifierQuotaDisabled(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	verifier := NewOTPVerifier(client)
	ctx := context.Background()

	lease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 0, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() disabled error = %v", err)
	}
	if !ok {
		t.Fatalf("expected TryConsume() with limit<=0 to always allow")
	}
	if lease.Member != "" {
		t.Fatalf("disabled quota should not return a rollback lease: %#v", lease)
	}
}

func TestOTPVerifierQuotaSetsTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	verifier := NewOTPVerifier(client)
	ctx := context.Background()
	window := time.Hour

	lease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 2, window)
	if err != nil {
		t.Fatalf("TryConsume() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected TryConsume() to be allowed")
	}
	key := otpSendQuotaRedisKey(lease.PhoneE164, lease.Scene, lease.Dimension)
	ttl := mr.TTL(key)
	if ttl <= 0 || ttl > window {
		t.Fatalf("quota key TTL = %s, want within (0, %s]", ttl, window)
	}
}

func TestOTPVerifierQuotaUsesSlidingWindowAcrossFixedBoundary(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	now := time.Date(2026, 6, 4, 12, 59, 59, 0, time.UTC)
	firstSendAt := now
	verifier := newOTPVerifierWithClock(client, func() time.Time { return now })
	ctx := context.Background()

	_, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 1, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() first member error = %v", err)
	}
	if !ok {
		t.Fatalf("expected first member consume to be allowed")
	}

	now = now.Add(time.Minute)
	_, ok, err = verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 1, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() across fixed boundary error = %v", err)
	}
	if ok {
		t.Fatalf("expected consume one minute after a fixed hour boundary to remain limited by the sliding hour")
	}

	now = firstSendAt.Add(time.Hour)
	_, ok, err = verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 1, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() after sliding window elapsed error = %v", err)
	}
	if !ok {
		t.Fatalf("expected consume to be allowed after the first send slides out of the window")
	}
}

func TestOTPVerifierRollbackRemovesOriginalSlidingWindowMember(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	now := time.Date(2026, 6, 4, 12, 59, 59, 0, time.UTC)
	verifier := newOTPVerifierWithClock(client, func() time.Time { return now })
	ctx := context.Background()

	oldLease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 1, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() old member error = %v", err)
	}
	if !ok {
		t.Fatalf("expected old member consume to be allowed")
	}
	key := otpSendQuotaRedisKey(oldLease.PhoneE164, oldLease.Scene, oldLease.Dimension)

	if err := verifier.Rollback(ctx, oldLease); err != nil {
		t.Fatalf("Rollback() old lease error = %v", err)
	}
	if mr.Exists(key) {
		t.Fatalf("expected quota key %q to be removed after rolling back its only member", key)
	}
}

func TestOTPVerifierRollbackRemovesOnlyLeasedMember(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	verifier := newOTPVerifierWithClock(client, func() time.Time { return now })
	ctx := context.Background()

	firstLease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 2, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() first error = %v", err)
	}
	if !ok {
		t.Fatalf("expected first consume to be allowed")
	}
	now = now.Add(time.Minute)
	secondLease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 2, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() second error = %v", err)
	}
	if !ok {
		t.Fatalf("expected second consume to be allowed")
	}
	key := otpSendQuotaRedisKey(firstLease.PhoneE164, firstLease.Scene, firstLease.Dimension)

	if err := verifier.Rollback(ctx, firstLease); err != nil {
		t.Fatalf("Rollback() first lease error = %v", err)
	}
	count, err := client.ZCard(ctx, key).Result()
	if err != nil {
		t.Fatalf("ZCard() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("quota member count = %d, want 1", count)
	}
	score, err := client.ZScore(ctx, key, secondLease.Member).Result()
	if err != nil {
		t.Fatalf("expected second lease member to remain: %v", err)
	}
	if score != float64(now.UnixMilli()) {
		t.Fatalf("second member score = %f, want %d", score, now.UnixMilli())
	}
}

func TestOTPVerifierRollbackIsSafeForEmptyOrMissingLease(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	verifier := NewOTPVerifier(client)
	ctx := context.Background()

	if err := verifier.Rollback(ctx, challengeApp.OTPSendQuotaLease{}); err != nil {
		t.Fatalf("Rollback() empty lease error = %v", err)
	}
	if err := verifier.Rollback(ctx, challengeApp.OTPSendQuotaLease{
		PhoneE164: "+8613800138000",
		Scene:     "login",
		Dimension: "hourly",
		Member:    "missing",
		Window:    time.Hour,
	}); err != nil {
		t.Fatalf("Rollback() missing key error = %v", err)
	}
}
