package redis

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestOTPVerifierVerifyAndConsume(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	verifier := NewOTPVerifier(client)
	ctx := context.Background()

	if err := verifier.Put(ctx, "+8613800138000", "login", "123456", time.Minute); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	rawOTPKey := otpRedisKey("+8613800138000", "login", "123456")
	if !mr.Exists(rawOTPKey) {
		t.Fatalf("expected raw redis key %q to exist", rawOTPKey)
	}

	if ok := verifier.VerifyAndConsume(ctx, "+8613800138000", "login", "123456"); !ok {
		t.Fatalf("expected first VerifyAndConsume() to succeed")
	}
	if mr.Exists(rawOTPKey) {
		t.Fatalf("expected OTP key %q to be consumed", rawOTPKey)
	}
	if ok := verifier.VerifyAndConsume(ctx, "+8613800138000", "login", "123456"); ok {
		t.Fatalf("expected second VerifyAndConsume() to fail")
	}
}

func TestOTPVerifierDelete(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	verifier := NewOTPVerifier(client)
	ctx := context.Background()

	if err := verifier.Put(ctx, "+8613800138000", "login", "654321", time.Minute); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	rawOTPKey := otpRedisKey("+8613800138000", "login", "654321")

	if err := verifier.Delete(ctx, "+8613800138000", "login", "654321"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if mr.Exists(rawOTPKey) {
		t.Fatalf("expected OTP key %q to be deleted", rawOTPKey)
	}
	if ok := verifier.VerifyAndConsume(ctx, "+8613800138000", "login", "654321"); ok {
		t.Fatalf("deleted OTP should not be verifiable")
	}
}

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
	var lease authentication.OTPSendQuotaLease
	for i := 1; i <= limit; i++ {
		gotLease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", limit, time.Hour)
		if err != nil {
			t.Fatalf("TryConsume() #%d error = %v", i, err)
		}
		if !ok {
			t.Fatalf("expected TryConsume() #%d to be allowed within limit", i)
		}
		if gotLease.Bucket == "" {
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
	if lease.Bucket != "" {
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
	key := otpSendQuotaRedisKey(lease.PhoneE164, lease.Scene, lease.Dimension, lease.Bucket)
	ttl := mr.TTL(key)
	if ttl <= 0 || ttl > window {
		t.Fatalf("quota key TTL = %s, want within (0, %s]", ttl, window)
	}
}

func TestOTPVerifierRollbackUsesOriginalQuotaBucketAcrossWindowBoundary(t *testing.T) {
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
		t.Fatalf("TryConsume() old bucket error = %v", err)
	}
	if !ok {
		t.Fatalf("expected old bucket consume to be allowed")
	}
	oldKey := otpSendQuotaRedisKey(oldLease.PhoneE164, oldLease.Scene, oldLease.Dimension, oldLease.Bucket)

	now = now.Add(time.Minute)
	newLease, ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 1, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() new bucket error = %v", err)
	}
	if !ok {
		t.Fatalf("expected new bucket consume to be allowed")
	}
	newKey := otpSendQuotaRedisKey(newLease.PhoneE164, newLease.Scene, newLease.Dimension, newLease.Bucket)
	if oldKey == newKey {
		t.Fatalf("expected distinct quota buckets across the hour boundary")
	}

	if err := verifier.Rollback(ctx, oldLease); err != nil {
		t.Fatalf("Rollback() old lease error = %v", err)
	}
	if mr.Exists(oldKey) {
		t.Fatalf("expected old quota bucket %q to be removed after rollback", oldKey)
	}
	if !mr.Exists(newKey) {
		t.Fatalf("expected new quota bucket %q to remain after old lease rollback", newKey)
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

	if err := verifier.Rollback(ctx, authentication.OTPSendQuotaLease{}); err != nil {
		t.Fatalf("Rollback() empty lease error = %v", err)
	}
	if err := verifier.Rollback(ctx, authentication.OTPSendQuotaLease{
		PhoneE164: "+8613800138000",
		Scene:     "login",
		Dimension: "hourly",
		Bucket:    "missing",
		Window:    time.Hour,
	}); err != nil {
		t.Fatalf("Rollback() missing key error = %v", err)
	}
}
