package redis

import (
	"context"
	"fmt"

	redisinfra "github.com/redis/go-redis/v9"
	wechatcache "github.com/silenceper/wechat/v2/cache"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
)

type redisFamilyInspector struct {
	family cachegovernance.Family
	client *redisinfra.Client
	notes  []string
}

func newRedisFamilyInspector(family cachegovernance.Family, client *redisinfra.Client, notes ...string) cachegovernance.FamilyInspector {
	return &redisFamilyInspector{
		family: family,
		client: client,
		notes:  notes,
	}
}

func (i *redisFamilyInspector) Descriptor() cachegovernance.FamilyDescriptor {
	descriptor, ok := cachegovernance.GetFamily(i.family)
	if !ok {
		return cachegovernance.FamilyDescriptor{
			Family:    i.family,
			Backend:   cachegovernance.BackendKindRedis,
			RedisType: cachegovernance.RedisDataTypeString,
		}
	}
	return descriptor
}

func (i *redisFamilyInspector) Status(ctx context.Context) (cachegovernance.FamilyStatus, error) {
	status := cachegovernance.FamilyStatus{
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
func RedisStoreInspectors(store *RedisStore) []cachegovernance.FamilyInspector {
	if store == nil {
		return nil
	}
	return store.FamilyInspectors()
}

// SessionStoreInspectors 返回 SessionStore 对应的缓存族状态读取器。
func SessionStoreInspectors(store *SessionStore) []cachegovernance.FamilyInspector {
	if store == nil {
		return nil
	}
	return store.FamilyInspectors()
}

// OTPVerifierInspectors 返回 OTP 适配器对应的缓存族状态读取器。
func OTPVerifierInspectors(verifier *OTPVerifierImpl) []cachegovernance.FamilyInspector {
	if verifier == nil {
		return nil
	}
	return verifier.FamilyInspectors()
}

// AccessTokenCacheInspectors 返回微信 access token 缓存对应的状态读取器。
func AccessTokenCacheInspectors(cache wechatapp.AccessTokenCache) []cachegovernance.FamilyInspector {
	typed, ok := cache.(*accessTokenCache)
	if !ok || typed == nil {
		return nil
	}
	return typed.FamilyInspectors()
}

// WechatSDKCacheInspectors 返回微信 SDK 缓存对应的状态读取器。
func WechatSDKCacheInspectors(cache wechatcache.Cache) []cachegovernance.FamilyInspector {
	typed, ok := cache.(*WechatSDKCache)
	if !ok || typed == nil {
		return nil
	}
	return typed.FamilyInspectors()
}
