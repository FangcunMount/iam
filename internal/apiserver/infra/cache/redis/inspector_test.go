package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisAdapterFamilyInspectors(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx := context.Background()
	tokenStore := NewRedisStore(client)
	otpVerifier := NewOTPVerifier(client)
	accessTokenCache := NewAccessTokenCache(client).(*accessTokenCache)
	wechatSDKCache := NewWechatSDKCache(client).(*WechatSDKCache)

	familyInspectors := append(tokenStore.FamilyInspectors(), otpVerifier.FamilyInspectors()...)
	familyInspectors = append(familyInspectors, accessTokenCache.FamilyInspectors()...)
	familyInspectors = append(familyInspectors, wechatSDKCache.FamilyInspectors()...)

	if len(familyInspectors) != 6 {
		t.Fatalf("inspector count = %d, want 6", len(familyInspectors))
	}

	for _, inspector := range familyInspectors {
		status, err := inspector.Status(ctx)
		if err != nil {
			t.Fatalf("Status(%s) error = %v", inspector.Descriptor().Family, err)
		}
		if !status.Configured {
			t.Fatalf("Status(%s).Configured = false, want true", inspector.Descriptor().Family)
		}
		if !status.Healthy {
			t.Fatalf("Status(%s).Healthy = false, want true; notes=%v", inspector.Descriptor().Family, status.Notes)
		}
	}
}
