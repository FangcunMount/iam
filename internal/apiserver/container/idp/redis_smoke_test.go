package idp

import (
	"context"
	"fmt"
	"testing"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v2/internal/apiserver/cache"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestModuleInitializeWithRedisAdapters(t *testing.T) {
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
		t.Fatalf("Module.Initialize() error = %v", err)
	}

	if module.WechatAppTokenService == nil {
		t.Fatalf("expected WechatAppTokenService to be initialized")
	}
	if module.ApplicationCapabilities().WechatAppService == nil {
		t.Fatalf("expected WechatAppService capability to be initialized")
	}
	assertInspectorFamilies(t, module.CacheFamilyInspectors(), []cachemodel.Family{
		cachemodel.FamilyIDPWechatAccessToken,
		cachemodel.FamilyIDPWechatSDK,
	})
}

func assertInspectorFamilies(t *testing.T, inspectors []cachegovernance.FamilyInspector, want []cachemodel.Family) {
	t.Helper()
	if err := validateInspectorFamilies(inspectors, want); err != nil {
		t.Fatalf("CacheFamilyInspectors() families mismatch: %v", err)
	}
}

func validateInspectorFamilies(inspectors []cachegovernance.FamilyInspector, want []cachemodel.Family) error {
	seen := make(map[cachemodel.Family]struct{}, len(inspectors))
	for index, inspector := range inspectors {
		if inspector == nil {
			return fmt.Errorf("inspector[%d] is nil", index)
		}
		family := inspector.Descriptor().Family
		if _, ok := cachemodel.GetFamily(family); !ok {
			return fmt.Errorf("unknown cache family %s", family)
		}
		if _, ok := seen[family]; ok {
			return fmt.Errorf("duplicate cache family %s", family)
		}
		seen[family] = struct{}{}
	}
	if len(seen) != len(want) {
		return fmt.Errorf("family count = %d, want %d", len(seen), len(want))
	}
	for _, family := range want {
		if _, ok := seen[family]; !ok {
			return fmt.Errorf("missing cache family %s", family)
		}
	}
	return nil
}

type inspectorFamilyStub struct {
	family cachemodel.Family
}

func (s inspectorFamilyStub) Descriptor() cachegovernance.FamilyDescriptor {
	return cachegovernance.FamilyDescriptor{Family: s.family}
}

func (s inspectorFamilyStub) Status(context.Context) (cachegovernance.FamilyStatus, error) {
	return cachegovernance.FamilyStatus{Family: s.family}, nil
}
