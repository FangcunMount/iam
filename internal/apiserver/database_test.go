package apiserver

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/FangcunMount/iam/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
	"github.com/alicebob/miniredis/v2"
)

func TestDatabaseManagerInitializeWithFoundationCacheRedis(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(t, mr)
	dm := NewDatabaseManager(cfg)

	if err := dm.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		_ = dm.Close()
	})

	client, err := dm.GetCacheRedisClient()
	if err != nil {
		t.Fatalf("GetCacheRedisClient() error = %v", err)
	}

	ctx := context.Background()
	if err := client.Set(ctx, "iam:test:redis", "ok", 0).Err(); err != nil {
		t.Fatalf("redis Set() error = %v", err)
	}
	got, err := mr.Get("iam:test:redis")
	if err != nil {
		t.Fatalf("miniredis Get() error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("redis raw value = %q, want %q", got, "ok")
	}

	if err := dm.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

func TestDatabaseManagerGetCacheRedisClientWhenRedisNotConfigured(t *testing.T) {
	opts := apiserveroptions.NewOptions()
	opts.MySQLOptions.Host = ""
	opts.RedisOptions.Cache.Host = ""
	opts.RedisOptions.Cache.Addrs = nil
	cfg, err := config.CreateConfigFromOptions(opts)
	if err != nil {
		t.Fatalf("CreateConfigFromOptions() error = %v", err)
	}

	dm := NewDatabaseManager(cfg)
	if err := dm.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		_ = dm.Close()
	})

	if _, err := dm.GetCacheRedisClient(); err == nil {
		t.Fatalf("GetCacheRedisClient() should fail when redis is not configured")
	}
}

func newTestConfig(t *testing.T, mr *miniredis.Miniredis) *config.Config {
	t.Helper()

	opts := apiserveroptions.NewOptions()
	opts.MySQLOptions.Host = ""
	opts.RedisOptions.Cache.Addrs = nil

	host, port := splitRedisAddr(t, mr.Addr())
	opts.RedisOptions.Cache.Host = host
	opts.RedisOptions.Cache.Port = port
	opts.RedisOptions.Cache.Database = 0
	opts.RedisOptions.Cache.EnableCluster = false
	opts.RedisOptions.Cache.EnableLogging = false

	cfg, err := config.CreateConfigFromOptions(opts)
	if err != nil {
		t.Fatalf("CreateConfigFromOptions() error = %v", err)
	}

	return cfg
}

func splitRedisAddr(t *testing.T, addr string) (string, int) {
	t.Helper()

	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", addr, err)
	}

	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", portText, err)
	}

	return host, port
}
