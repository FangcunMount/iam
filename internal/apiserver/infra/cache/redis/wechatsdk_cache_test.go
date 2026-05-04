package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	wechatCache "github.com/silenceper/wechat/v2/cache"
)

func TestWechatSDKCacheLifecycle(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	cache := NewWechatSDKCache(client)
	key := "wechat:sdk:token:app-1"

	if err := cache.Set(key, "sdk-token", time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !mr.Exists(key) {
		t.Fatalf("expected raw redis key %q to exist", key)
	}
	rawValue, err := mr.Get(key)
	if err != nil {
		t.Fatalf("miniredis Get(%q) error = %v", key, err)
	}
	if rawValue != "sdk-token" {
		t.Fatalf("stored redis value = %q, want %q", rawValue, "sdk-token")
	}

	got := cache.Get(key)
	if got == nil {
		t.Fatalf("Get() = nil, want cached value")
	}
	if got.(string) != "sdk-token" {
		t.Fatalf("Get() = %q, want %q", got.(string), "sdk-token")
	}
	if !cache.IsExist(key) {
		t.Fatalf("IsExist() should report existing key")
	}

	if err := cache.Delete(key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if mr.Exists(key) {
		t.Fatalf("expected raw redis key %q to be deleted", key)
	}
	if cache.IsExist(key) {
		t.Fatalf("IsExist() should report deleted key as missing")
	}
}

func TestWechatSDKCacheContextMethods(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	cache := NewWechatSDKCache(client)
	contextCache, ok := cache.(wechatCache.ContextCache)
	if !ok {
		t.Fatalf("NewWechatSDKCache() should implement wechat cache.ContextCache")
	}

	ctx := context.Background()
	key := "wechat:sdk:ticket:app-2"

	if err := contextCache.SetContext(ctx, key, []byte("ticket-value"), time.Minute); err != nil {
		t.Fatalf("SetContext() error = %v", err)
	}
	got := contextCache.GetContext(ctx, key)
	if got == nil {
		t.Fatalf("GetContext() = nil, want cached value")
	}
	if got.(string) != "ticket-value" {
		t.Fatalf("GetContext() = %q, want %q", got.(string), "ticket-value")
	}
	if !contextCache.IsExistContext(ctx, key) {
		t.Fatalf("IsExistContext() should report existing key")
	}

	if err := contextCache.DeleteContext(ctx, key); err != nil {
		t.Fatalf("DeleteContext() error = %v", err)
	}
	if contextCache.GetContext(ctx, key) != nil {
		t.Fatalf("GetContext() after delete should return nil")
	}
}
