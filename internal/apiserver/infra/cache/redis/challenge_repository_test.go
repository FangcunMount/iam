package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
)

func TestChallengeRepositoryCreateGetConsume(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	repo := NewChallengeRepository(client)
	ctx := context.Background()
	challenge := &challengeDomain.AuthChallenge{
		ID:         "sms_otp:login:+8613800138000",
		Type:       challengeDomain.TypeSMSOTP,
		Scene:      "login",
		Target:     "+8613800138000",
		SecretHash: []byte("hash"),
		Payload: map[string]string{
			"app_id": "wx-app",
		},
		ExpiresAt: time.Now().Add(time.Minute),
		CreatedAt: time.Now(),
	}

	if err := repo.Create(ctx, challenge); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	rawKey := challengeRedisKey(challenge.ID)
	if !mr.Exists(rawKey) {
		t.Fatalf("expected challenge key %q to exist", rawKey)
	}

	loaded, err := repo.Get(ctx, challenge.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded == nil || loaded.ID != challenge.ID {
		t.Fatalf("loaded challenge = %#v", loaded)
	}
	if loaded.Payload["app_id"] != "wx-app" {
		t.Fatalf("payload app_id = %q", loaded.Payload["app_id"])
	}

	consumed, err := repo.ConsumeIfSecretMatches(ctx, challenge.ID, challenge.SecretHash)
	if err != nil {
		t.Fatalf("ConsumeIfSecretMatches() error = %v", err)
	}
	if !consumed {
		t.Fatal("ConsumeIfSecretMatches() = false, want true")
	}
	if mr.Exists(rawKey) {
		t.Fatalf("expected challenge key %q to be consumed", rawKey)
	}
}

func TestChallengeRepositoryConsumeIfSecretMatchesIsSingleUse(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	repo := NewChallengeRepository(client)
	ctx := context.Background()
	challenge := &challengeDomain.AuthChallenge{
		ID:         "sms_otp:login:+8613800138000",
		Type:       challengeDomain.TypeSMSOTP,
		SecretHash: []byte("expected-hash"),
		ExpiresAt:  time.Now().Add(time.Minute),
		CreatedAt:  time.Now(),
	}
	if err := repo.Create(ctx, challenge); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	type result struct {
		consumed bool
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			ok, err := repo.ConsumeIfSecretMatches(ctx, challenge.ID, challenge.SecretHash)
			results <- result{consumed: ok, err: err}
		}()
	}
	close(start)

	successes := 0
	for range 2 {
		got := <-results
		if got.err != nil {
			t.Fatalf("ConsumeIfSecretMatches() error = %v", got.err)
		}
		if got.consumed {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful consumes = %d, want 1", successes)
	}
}

func TestChallengeRepositoryStaleSecretDoesNotConsumeReplacement(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	repo := NewChallengeRepository(client)
	ctx := context.Background()
	challenge := &challengeDomain.AuthChallenge{
		ID:         "sms_otp:login:+8613800138000",
		Type:       challengeDomain.TypeSMSOTP,
		SecretHash: []byte("replacement-hash"),
		ExpiresAt:  time.Now().Add(time.Minute),
		CreatedAt:  time.Now(),
	}
	if err := repo.Create(ctx, challenge); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	consumed, err := repo.ConsumeIfSecretMatches(ctx, challenge.ID, []byte("stale-hash"))
	if err != nil {
		t.Fatalf("ConsumeIfSecretMatches() error = %v", err)
	}
	if consumed {
		t.Fatal("stale secret consumed replacement challenge")
	}
	if !mr.Exists(challengeRedisKey(challenge.ID)) {
		t.Fatal("replacement challenge was deleted")
	}
}

func TestChallengeRepositoryFailedAttemptsExhaustCurrentChallenge(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	repo := NewChallengeRepository(client)
	ctx := context.Background()
	challenge := &challengeDomain.AuthChallenge{
		ID:         "sms_otp:login:+8613800138000",
		Type:       challengeDomain.TypeSMSOTP,
		SecretHash: []byte("current-hash"),
		ExpiresAt:  time.Now().Add(time.Minute),
		CreatedAt:  time.Now(),
	}
	if err := repo.Create(ctx, challenge); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		current, exhausted, err := repo.RecordFailedAttemptIfCurrent(ctx, challenge.ID, challenge.SecretHash, 5)
		if err != nil {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
		if !current {
			t.Fatalf("attempt %d did not match current challenge", attempt)
		}
		if got, want := exhausted, attempt == 5; got != want {
			t.Fatalf("attempt %d exhausted = %t, want %t", attempt, got, want)
		}
	}
	if mr.Exists(challengeRedisKey(challenge.ID)) {
		t.Fatal("challenge still exists after maximum attempts")
	}
}

func TestChallengeRepositoryStaleFailureDoesNotAffectReplacement(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	repo := NewChallengeRepository(client)
	ctx := context.Background()
	challenge := &challengeDomain.AuthChallenge{
		ID:         "sms_otp:login:+8613800138000",
		Type:       challengeDomain.TypeSMSOTP,
		SecretHash: []byte("replacement-hash"),
		ExpiresAt:  time.Now().Add(time.Minute),
		CreatedAt:  time.Now(),
	}
	if err := repo.Create(ctx, challenge); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	current, exhausted, err := repo.RecordFailedAttemptIfCurrent(ctx, challenge.ID, []byte("stale-hash"), 1)
	if err != nil {
		t.Fatalf("RecordFailedAttemptIfCurrent() error = %v", err)
	}
	if current || exhausted {
		t.Fatalf("stale failure current=%t exhausted=%t", current, exhausted)
	}
	if !mr.Exists(challengeRedisKey(challenge.ID)) {
		t.Fatal("replacement challenge was deleted")
	}
}
