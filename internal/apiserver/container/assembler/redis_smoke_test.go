package assembler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v2/internal/apiserver/cache"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
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
	assertInspectorFamilies(t, module.CacheFamilyInspectors(), []cachemodel.Family{
		cachemodel.FamilyAuthnRefreshToken,
		cachemodel.FamilyAuthnRevokedAccessToken,
		cachemodel.FamilyAuthnSession,
		cachemodel.FamilyAuthnUserSessionIndex,
		cachemodel.FamilyAuthnAccountSessionIndex,
		cachemodel.FamilyAuthnLoginOTP,
		cachemodel.FamilyAuthnLoginOTPSendGate,
		cachemodel.FamilyAuthnJWKSPublishSnapshot,
	})
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
	assertInspectorFamilies(t, module.CacheFamilyInspectors(), []cachemodel.Family{
		cachemodel.FamilyIDPWechatAccessToken,
		cachemodel.FamilyIDPWechatSDK,
	})
}

func TestValidateInspectorFamiliesRejectsDuplicateAndUnknown(t *testing.T) {
	if err := validateInspectorFamilies([]cachegovernance.FamilyInspector{
		inspectorFamilyStub{family: cachemodel.FamilyAuthnRefreshToken},
		inspectorFamilyStub{family: cachemodel.FamilyAuthnRefreshToken},
	}, []cachemodel.Family{cachemodel.FamilyAuthnRefreshToken}); err == nil || !strings.Contains(err.Error(), "duplicate cache family") {
		t.Fatalf("duplicate validation error = %v, want duplicate cache family", err)
	}

	if err := validateInspectorFamilies([]cachegovernance.FamilyInspector{
		inspectorFamilyStub{family: "unknown.family"},
	}, []cachemodel.Family{"unknown.family"}); err == nil || !strings.Contains(err.Error(), "unknown cache family") {
		t.Fatalf("unknown validation error = %v, want unknown cache family", err)
	}
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
