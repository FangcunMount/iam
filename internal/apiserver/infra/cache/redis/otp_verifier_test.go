package redis

import (
	"context"
	"testing"
	"time"

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
	for i := 1; i <= limit; i++ {
		ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", limit, time.Hour)
		if err != nil {
			t.Fatalf("TryConsume() #%d error = %v", i, err)
		}
		if !ok {
			t.Fatalf("expected TryConsume() #%d to be allowed within limit", i)
		}
	}

	ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", limit, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() over-limit error = %v", err)
	}
	if ok {
		t.Fatalf("expected TryConsume() to be rejected when exceeding limit")
	}

	// 回退一次后，应重新允许一次发送。
	if err := verifier.Rollback(ctx, "+8613800138000", "login", "hourly", time.Hour); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	ok, err = verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", limit, time.Hour)
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

	ok, err := verifier.TryConsume(ctx, "+8613800138000", "login", "hourly", 0, time.Hour)
	if err != nil {
		t.Fatalf("TryConsume() disabled error = %v", err)
	}
	if !ok {
		t.Fatalf("expected TryConsume() with limit<=0 to always allow")
	}
}
