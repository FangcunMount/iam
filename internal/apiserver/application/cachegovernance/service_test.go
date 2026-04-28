package cachegovernance

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	jwksdomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	cacheinfra "github.com/FangcunMount/iam/internal/apiserver/infra/cache"
	redisinfra "github.com/FangcunMount/iam/internal/apiserver/infra/redis"
)

type governanceJWKSRepository struct {
	publishable []*jwksdomain.Key
}

func (r *governanceJWKSRepository) Save(context.Context, *jwksdomain.Key) error   { return nil }
func (r *governanceJWKSRepository) Update(context.Context, *jwksdomain.Key) error { return nil }
func (r *governanceJWKSRepository) Delete(context.Context, string) error          { return nil }
func (r *governanceJWKSRepository) FindByKid(context.Context, string) (*jwksdomain.Key, error) {
	return nil, nil
}
func (r *governanceJWKSRepository) FindByStatus(context.Context, jwksdomain.KeyStatus) ([]*jwksdomain.Key, error) {
	return nil, nil
}
func (r *governanceJWKSRepository) FindPublishable(context.Context) ([]*jwksdomain.Key, error) {
	return r.publishable, nil
}
func (r *governanceJWKSRepository) FindExpired(context.Context) ([]*jwksdomain.Key, error) {
	return nil, nil
}
func (r *governanceJWKSRepository) FindAll(context.Context, int, int) ([]*jwksdomain.Key, int64, error) {
	return nil, 0, nil
}
func (r *governanceJWKSRepository) CountByStatus(context.Context, jwksdomain.KeyStatus) (int64, error) {
	return 0, nil
}

func TestReadServiceOverview(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx := context.Background()
	tokenStore := redisinfra.NewRedisStore(client)
	sessionStore := redisinfra.NewSessionStore(client)
	otpVerifier := redisinfra.NewOTPVerifier(client)
	accessTokenCache := redisinfra.NewAccessTokenCache(client)
	wechatSDKCache := redisinfra.NewWechatSDKCache(client)

	jwksRepo := &governanceJWKSRepository{
		publishable: []*jwksdomain.Key{
			jwksdomain.NewKey("kid-1", jwksdomain.PublicJWK{
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				Kid: "kid-1",
				N:   mustStr("n"),
				E:   mustStr("e"),
			}),
		},
	}
	keySetBuilder := jwksdomain.NewKeySetBuilder(jwksRepo)
	if _, _, err := keySetBuilder.BuildJWKS(ctx); err != nil {
		t.Fatalf("BuildJWKS() error = %v", err)
	}

	inspectors := append(redisinfra.RedisStoreInspectors(tokenStore), redisinfra.OTPVerifierInspectors(otpVerifier)...)
	inspectors = append(inspectors, redisinfra.SessionStoreInspectors(sessionStore)...)
	inspectors = append(inspectors, redisinfra.AccessTokenCacheInspectors(accessTokenCache)...)
	inspectors = append(inspectors, redisinfra.WechatSDKCacheInspectors(wechatSDKCache)...)
	inspectors = append(inspectors, NewJWKSPublishSnapshotInspector(keySetBuilder))

	service := NewReadService(inspectors)
	overview, err := service.Overview(ctx)
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}

	if len(overview.Families) != 10 {
		t.Fatalf("family count = %d, want 10", len(overview.Families))
	}
	if len(overview.RuntimeStatuses) != 2 {
		t.Fatalf("runtime status count = %d, want 2", len(overview.RuntimeStatuses))
	}

	for _, view := range overview.Families {
		if !view.Status.Configured {
			t.Fatalf("family %s configured = false, want true", view.Descriptor.Family)
		}
		if !view.Status.Healthy {
			t.Fatalf("family %s healthy = false, want true; notes=%v", view.Descriptor.Family, view.Status.Notes)
		}
	}

	redisRuntime := findRuntimeStatus(t, overview.RuntimeStatuses, cacheinfra.BackendKindRedis)
	if !redisRuntime.Configured || !redisRuntime.Healthy {
		t.Fatalf("redis runtime = %#v, want configured and healthy", redisRuntime)
	}
	memoryRuntime := findRuntimeStatus(t, overview.RuntimeStatuses, cacheinfra.BackendKindMemory)
	if !memoryRuntime.Configured || !memoryRuntime.Healthy {
		t.Fatalf("memory runtime = %#v, want configured and healthy", memoryRuntime)
	}
}

func TestReadServiceDegradesWithoutInspectors(t *testing.T) {
	service := NewReadService(nil)

	overview, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if len(overview.Families) != 10 {
		t.Fatalf("family count = %d, want 10", len(overview.Families))
	}

	for _, view := range overview.Families {
		if view.Status.Configured {
			t.Fatalf("family %s configured = true, want false when inspector missing", view.Descriptor.Family)
		}
		if view.Status.Healthy {
			t.Fatalf("family %s healthy = true, want false when inspector missing", view.Descriptor.Family)
		}
	}

	redisRuntime := findRuntimeStatus(t, overview.RuntimeStatuses, cacheinfra.BackendKindRedis)
	if redisRuntime.Configured || redisRuntime.Healthy {
		t.Fatalf("redis runtime = %#v, want unconfigured and unhealthy", redisRuntime)
	}
}

func findRuntimeStatus(t *testing.T, statuses []cacheinfra.RuntimeStatus, backend cacheinfra.BackendKind) cacheinfra.RuntimeStatus {
	t.Helper()
	for _, status := range statuses {
		if status.Backend == backend {
			return status
		}
	}
	t.Fatalf("runtime status for backend %s not found", backend)
	return cacheinfra.RuntimeStatus{}
}

func mustStr(s string) *string {
	return &s
}
