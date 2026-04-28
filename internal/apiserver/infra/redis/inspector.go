package redis

import (
	"context"
	"fmt"

	redisinfra "github.com/redis/go-redis/v9"
	wechatcache "github.com/silenceper/wechat/v2/cache"

	"github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	cacheinfra "github.com/FangcunMount/iam/internal/apiserver/infra/cache"
)

type redisFamilyInspector struct {
	family cacheinfra.Family
	client *redisinfra.Client
	notes  []string
}

func newRedisFamilyInspector(family cacheinfra.Family, client *redisinfra.Client, notes ...string) cacheinfra.FamilyInspector {
	return &redisFamilyInspector{
		family: family,
		client: client,
		notes:  notes,
	}
}

func (i *redisFamilyInspector) Descriptor() cacheinfra.FamilyDescriptor {
	descriptor, ok := cacheinfra.GetFamily(i.family)
	if !ok {
		return cacheinfra.FamilyDescriptor{
			Family:    i.family,
			Backend:   cacheinfra.BackendKindRedis,
			RedisType: cacheinfra.RedisDataTypeString,
		}
	}
	return descriptor
}

func (i *redisFamilyInspector) Status(ctx context.Context) (cacheinfra.FamilyStatus, error) {
	status := cacheinfra.FamilyStatus{
		Family:          i.family,
		Configured:      i.client != nil,
		Healthy:         false,
		EntryCountKnown: false,
		Notes:           append([]string{}, i.notes...),
	}

	if i.client == nil {
		status.Notes = append(status.Notes, "Redis 客户端未配置。")
		return status, nil
	}

	if err := i.client.Ping(ctx).Err(); err != nil {
		status.Notes = append(status.Notes, fmt.Sprintf("Redis 健康检查失败: %v", err))
		return status, nil
	}

	status.Healthy = true
	status.Notes = append(status.Notes, "Redis 客户端可用，当前只读治理未统计条目数量。")
	return status, nil
}

// RedisStoreInspectors 返回 RedisStore 对应的缓存族状态读取器。
func RedisStoreInspectors(store *RedisStore) []cacheinfra.FamilyInspector {
	if store == nil {
		return nil
	}
	return store.FamilyInspectors()
}

// SessionStoreInspectors 返回 SessionStore 对应的缓存族状态读取器。
func SessionStoreInspectors(store *SessionStore) []cacheinfra.FamilyInspector {
	if store == nil {
		return nil
	}
	return store.FamilyInspectors()
}

// OTPVerifierInspectors 返回 OTP 适配器对应的缓存族状态读取器。
func OTPVerifierInspectors(verifier *OTPVerifierImpl) []cacheinfra.FamilyInspector {
	if verifier == nil {
		return nil
	}
	return verifier.FamilyInspectors()
}

// AccessTokenCacheInspectors 返回微信 access token 缓存对应的状态读取器。
func AccessTokenCacheInspectors(cache wechatapp.AccessTokenCache) []cacheinfra.FamilyInspector {
	typed, ok := cache.(*accessTokenCache)
	if !ok || typed == nil {
		return nil
	}
	return typed.FamilyInspectors()
}

// WechatSDKCacheInspectors 返回微信 SDK 缓存对应的状态读取器。
func WechatSDKCacheInspectors(cache wechatcache.Cache) []cacheinfra.FamilyInspector {
	typed, ok := cache.(*WechatSDKCache)
	if !ok || typed == nil {
		return nil
	}
	return typed.FamilyInspectors()
}
