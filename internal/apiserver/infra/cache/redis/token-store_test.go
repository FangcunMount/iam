package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestRedisStoreRefreshTokenLifecycle(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := NewRedisStore(client)
	ctx := context.Background()
	refreshToken := tokenapp.NewRefreshToken(
		"rt-1",
		"refresh-value",
		"session-1",
		meta.FromUint64(1001),
		meta.FromUint64(2002),
		meta.FromUint64(3003),
		[]string{"pwd"},
		map[string]string{"device": "ios"},
		time.Hour,
	)

	if err := store.SaveRefreshToken(ctx, refreshToken); err != nil {
		t.Fatalf("SaveRefreshToken() error = %v", err)
	}

	rawKey := refreshTokenRedisKey(refreshToken.Value)
	if !mr.Exists(rawKey) {
		t.Fatalf("expected raw redis key %q to exist", rawKey)
	}
	rawValue, err := mr.Get(rawKey)
	if err != nil {
		t.Fatalf("miniredis Get(%q) error = %v", rawKey, err)
	}
	var payload refreshTokenData
	if err := json.Unmarshal([]byte(rawValue), &payload); err != nil {
		t.Fatalf("stored payload should be valid JSON: %v", err)
	}
	if payload.TokenID != refreshToken.ID {
		t.Fatalf("stored token_id = %q, want %q", payload.TokenID, refreshToken.ID)
	}
	if payload.SessionID != refreshToken.SessionID {
		t.Fatalf("stored session_id = %q, want %q", payload.SessionID, refreshToken.SessionID)
	}

	loaded, err := store.GetRefreshToken(ctx, refreshToken.Value)
	if err != nil {
		t.Fatalf("GetRefreshToken() error = %v", err)
	}
	if loaded == nil {
		t.Fatalf("GetRefreshToken() = nil, want token")
	}
	if loaded.ID != refreshToken.ID || loaded.Value != refreshToken.Value {
		t.Fatalf("loaded token = %#v, want id=%q value=%q", loaded, refreshToken.ID, refreshToken.Value)
	}

	if err := store.DeleteRefreshToken(ctx, refreshToken.Value); err != nil {
		t.Fatalf("DeleteRefreshToken() error = %v", err)
	}
	if mr.Exists(rawKey) {
		t.Fatalf("expected raw redis key %q to be deleted", rawKey)
	}
}

func TestRedisStoreRevokedAccessTokenLifecycle(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := NewRedisStore(client)
	ctx := context.Background()
	tokenID := "black-token"

	if err := store.MarkAccessTokenRevoked(ctx, tokenID, 30*time.Minute); err != nil {
		t.Fatalf("MarkAccessTokenRevoked() error = %v", err)
	}

	rawKey := revokedAccessTokenRedisKey(tokenID)
	if !mr.Exists(rawKey) {
		t.Fatalf("expected revoked access token key %q to exist", rawKey)
	}
	rawValue, err := mr.Get(rawKey)
	if err != nil {
		t.Fatalf("miniredis Get(%q) error = %v", rawKey, err)
	}
	if rawValue != "1" {
		t.Fatalf("revoked access token marker = %q, want %q", rawValue, "1")
	}

	revoked, err := store.IsAccessTokenRevoked(ctx, tokenID)
	if err != nil {
		t.Fatalf("IsAccessTokenRevoked() error = %v", err)
	}
	if !revoked {
		t.Fatalf("expected token to be marked revoked")
	}

	revoked, err = store.IsAccessTokenRevoked(ctx, "missing-token")
	if err != nil {
		t.Fatalf("IsAccessTokenRevoked() missing error = %v", err)
	}
	if revoked {
		t.Fatalf("expected missing token not to be marked revoked")
	}
}

func TestRedisStoreRejectsExpiredRefreshToken(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := NewRedisStore(client)
	ctx := context.Background()
	expiredToken := tokenapp.NewRefreshToken(
		"rt-expired",
		"expired-value",
		"session-expired",
		meta.FromUint64(1),
		meta.FromUint64(2),
		meta.FromUint64(3),
		nil,
		nil,
		-time.Second,
	)

	if err := store.SaveRefreshToken(ctx, expiredToken); err == nil {
		t.Fatalf("SaveRefreshToken() should reject expired token")
	}
	if mr.Exists(refreshTokenRedisKey(expiredToken.Value)) {
		t.Fatalf("expired token should not be written to redis")
	}
}

func TestRedisStoreRotateRefreshTokenIsSingleUse(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisStore(client)
	ctx := context.Background()
	oldToken := tokenapp.NewRefreshToken(
		"old-id", "old-value", "session-id",
		meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3),
		nil, nil, time.Hour,
	)
	if err := store.SaveRefreshToken(ctx, oldToken); err != nil {
		t.Fatalf("SaveRefreshToken() error = %v", err)
	}

	type result struct {
		index   int
		rotated bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for i := range 2 {
		go func(index int) {
			<-start
			candidate := tokenapp.NewRefreshToken(
				fmt.Sprintf("new-id-%d", index),
				fmt.Sprintf("new-value-%d", index),
				"session-id",
				meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3),
				nil, nil, time.Hour,
			)
			ok, err := store.RotateRefreshToken(ctx, oldToken.Value, oldToken.ID, candidate)
			results <- result{index: index, rotated: ok, err: err}
		}(i)
	}
	close(start)

	successes := 0
	winner := -1
	for range 2 {
		got := <-results
		if got.err != nil {
			t.Fatalf("RotateRefreshToken() error = %v", got.err)
		}
		if got.rotated {
			successes++
			winner = got.index
		}
	}
	if successes != 1 {
		t.Fatalf("successful rotations = %d, want 1", successes)
	}
	if loaded, err := store.GetRefreshToken(ctx, oldToken.Value); err != nil || loaded != nil {
		t.Fatalf("old refresh token = %#v, err = %v, want absent", loaded, err)
	}
	for i := range 2 {
		loaded, err := store.GetRefreshToken(ctx, fmt.Sprintf("new-value-%d", i))
		if err != nil {
			t.Fatalf("GetRefreshToken(candidate %d) error = %v", i, err)
		}
		if (loaded != nil) != (i == winner) {
			t.Fatalf("candidate %d exists = %t, winner = %d", i, loaded != nil, winner)
		}
	}
}

func TestRedisStoreRotateRefreshTokenRejectsMismatchedOldIDWithoutMutation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisStore(client)
	ctx := context.Background()
	oldToken := tokenapp.NewRefreshToken(
		"old-id", "old-value", "session-id",
		meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3),
		nil, nil, time.Hour,
	)
	candidate := tokenapp.NewRefreshToken(
		"new-id", "new-value", "session-id",
		meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3),
		nil, nil, time.Hour,
	)
	if err := store.SaveRefreshToken(ctx, oldToken); err != nil {
		t.Fatalf("SaveRefreshToken() error = %v", err)
	}

	rotated, err := store.RotateRefreshToken(ctx, oldToken.Value, "different-old-id", candidate)
	if err != nil {
		t.Fatalf("RotateRefreshToken() error = %v", err)
	}
	if rotated {
		t.Fatal("RotateRefreshToken() = true, want false")
	}
	if loaded, err := store.GetRefreshToken(ctx, oldToken.Value); err != nil || loaded == nil {
		t.Fatalf("old refresh token = %#v, err = %v, want preserved", loaded, err)
	}
	if loaded, err := store.GetRefreshToken(ctx, candidate.Value); err != nil || loaded != nil {
		t.Fatalf("new refresh token = %#v, err = %v, want absent", loaded, err)
	}
}

func TestRedisStoreReturnsErrorOnMalformedPayload(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store := NewRedisStore(client)
	ctx := context.Background()
	rawKey := refreshTokenRedisKey("broken-token")
	if err := mr.Set(rawKey, "{broken-json"); err != nil {
		t.Fatalf("miniredis Set(%q) error = %v", rawKey, err)
	}

	token, err := store.GetRefreshToken(ctx, "broken-token")
	if err == nil {
		t.Fatalf("GetRefreshToken() should fail on malformed payload")
	}
	if token != nil {
		t.Fatalf("GetRefreshToken() should return nil token on malformed payload")
	}
}
