package assembler

import (
	"testing"

	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthnModuleInitializeWithRedisAdapters(t *testing.T) {
	t.Setenv("TZ", "Asia/Shanghai")

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	mr := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	module := NewAuthnModule()
	if err := module.InitializeWithDeps(AuthnModuleDeps{
		DB:          db,
		RedisClient: redisClient,
		AppMode:     "test",
		Auth:        *apiserveroptions.NewAuthOptions(),
		JWKS:        *apiserveroptions.NewJWKSOptions(),
		SMS:         *apiserveroptions.NewSMSOptions(),
	}); err != nil {
		t.Fatalf("AuthnModule.Initialize() error = %v", err)
	}

	caps := module.ApplicationCapabilities()
	if caps.LoginPreparationService == nil {
		t.Fatalf("expected LoginPreparationService to be initialized")
	}
	if caps.TokenService == nil {
		t.Fatalf("expected TokenService to be initialized")
	}
	if got := len(module.CacheFamilyInspectors()); got != 8 {
		t.Fatalf("AuthnModule.CacheFamilyInspectors() count = %d, want 8", got)
	}
}

func TestIDPModuleInitializeWithRedisAdapters(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	mr := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	module := NewIDPModule()
	if err := module.InitializeWithDeps(IDPModuleDeps{
		DB:            db,
		RedisClient:   redisClient,
		EncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
	}); err != nil {
		t.Fatalf("IDPModule.Initialize() error = %v", err)
	}

	if module.WechatAppTokenService == nil {
		t.Fatalf("expected WechatAppTokenService to be initialized")
	}
	if module.ApplicationCapabilities().WechatAppService == nil {
		t.Fatalf("expected WechatAppService capability to be initialized")
	}
	if got := len(module.CacheFamilyInspectors()); got != 2 {
		t.Fatalf("IDPModule.CacheFamilyInspectors() count = %d, want 2", got)
	}
}
