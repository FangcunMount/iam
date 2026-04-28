package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
)

func TestAccessTokenCacheSetAndGet(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	cache := NewAccessTokenCache(client)
	ctx := context.Background()
	token := &wechatapp.AppAccessToken{
		Token:     "wechat-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := cache.Set(ctx, "app-1", token, time.Hour); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	rawKey := wechatAccessTokenRedisKey("app-1")
	if !mr.Exists(rawKey) {
		t.Fatalf("expected raw redis key %q to exist", rawKey)
	}

	loaded, err := cache.Get(ctx, "app-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded == nil {
		t.Fatalf("Get() = nil, want token")
	}
	if loaded.Token != token.Token {
		t.Fatalf("loaded token = %q, want %q", loaded.Token, token.Token)
	}

	missing, err := cache.Get(ctx, "missing-app")
	if err != nil {
		t.Fatalf("Get() missing error = %v", err)
	}
	if missing != nil {
		t.Fatalf("Get() missing = %#v, want nil", missing)
	}
}

func TestAccessTokenCacheTryLockRefresh(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	cache := NewAccessTokenCache(client)
	ctx := context.Background()
	rawLockKey := wechatAccessTokenLockRedisKey("app-1")

	ok, unlock, err := cache.TryLockRefresh(ctx, "app-1", 10*time.Second)
	if err != nil {
		t.Fatalf("TryLockRefresh() error = %v", err)
	}
	if !ok || unlock == nil {
		t.Fatalf("expected first lock acquisition to succeed")
	}
	if !mr.Exists(rawLockKey) {
		t.Fatalf("expected raw redis lock key %q to exist", rawLockKey)
	}

	ok, unlock2, err := cache.TryLockRefresh(ctx, "app-1", 10*time.Second)
	if err != nil {
		t.Fatalf("TryLockRefresh() second error = %v", err)
	}
	if ok || unlock2 != nil {
		t.Fatalf("expected second lock acquisition to fail while held")
	}

	unlock()
	if mr.Exists(rawLockKey) {
		t.Fatalf("expected raw redis lock key %q to be released", rawLockKey)
	}

	ok, unlock3, err := cache.TryLockRefresh(ctx, "app-1", 10*time.Second)
	if err != nil {
		t.Fatalf("TryLockRefresh() third error = %v", err)
	}
	if !ok || unlock3 == nil {
		t.Fatalf("expected lock acquisition after unlock to succeed")
	}
}
