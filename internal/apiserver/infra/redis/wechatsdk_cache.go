package redis

import (
	"context"
	"fmt"
	"time"

	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	"github.com/redis/go-redis/v9"
	wechatCache "github.com/silenceper/wechat/v2/cache"

	"github.com/FangcunMount/component-base/pkg/log"
	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
)

// WechatSDKCache 微信 SDK Cache 接口的 Redis 实现
// 适配 github.com/silenceper/wechat/v2/cache.Cache 接口。
// 微信 SDK 当前实际缓存的是 access token / js ticket 等字符串值，
// 因此这里迁到 Foundation 的 typed store，并保持 key 形态不变。
type WechatSDKCache struct {
	client *redis.Client
	store  *redisstore.ValueStore[string]
	ctx    context.Context
}

// NewWechatSDKCache 创建微信 SDK 缓存实例
func NewWechatSDKCache(client *redis.Client) wechatCache.Cache {
	return &WechatSDKCache{
		client: client,
		store:  newStringStore(client),
		ctx:    context.Background(),
	}
}

// FamilyInspectors 返回微信 SDK 缓存族的状态读取器。
func (c *WechatSDKCache) FamilyInspectors() []cachegovernance.FamilyInspector {
	return []cachegovernance.FamilyInspector{
		newRedisFamilyInspector(cachegovernance.FamilyIDPWechatSDK, c.client, "缓存值为微信 SDK 提供的字符串 token 或 ticket。"),
	}
}

// Get 从缓存中获取数据
func (c *WechatSDKCache) Get(key string) interface{} {
	return c.GetContext(c.ctx, key)
}

// GetContext 从缓存中获取数据。
func (c *WechatSDKCache) GetContext(ctx context.Context, key string) interface{} {
	storeKey, err := newStoreKey(key)
	if err != nil {
		return nil
	}
	val, found, err := c.store.Get(ctx, storeKey)
	if err != nil || !found {
		return nil
	}
	return val
}

// Set 设置缓存数据，timeout 单位为秒
func (c *WechatSDKCache) Set(key string, val interface{}, timeout time.Duration) error {
	return c.SetContext(c.ctx, key, val, timeout)
}

// SetContext 设置缓存数据。
func (c *WechatSDKCache) SetContext(ctx context.Context, key string, val interface{}, timeout time.Duration) error {
	storeKey, err := newStoreKey(key)
	if err != nil {
		return err
	}
	payload := stringifyWechatSDKValue(val)
	if err := c.store.Set(ctx, storeKey, payload, timeout); err != nil {
		return err
	}
	redisInfo(ctx, "wechat sdk cache set", log.String("key", key), log.Duration("ttl", timeout))
	return nil
}

// IsExist 判断 key 是否存在
func (c *WechatSDKCache) IsExist(key string) bool {
	return c.IsExistContext(c.ctx, key)
}

// IsExistContext 判断 key 是否存在。
func (c *WechatSDKCache) IsExistContext(ctx context.Context, key string) bool {
	storeKey, err := newStoreKey(key)
	if err != nil {
		return false
	}
	result, err := c.store.Exists(ctx, storeKey)
	if err != nil {
		return false
	}
	return result
}

// Delete 删除缓存
func (c *WechatSDKCache) Delete(key string) error {
	return c.DeleteContext(c.ctx, key)
}

// DeleteContext 删除缓存。
func (c *WechatSDKCache) DeleteContext(ctx context.Context, key string) error {
	storeKey, err := newStoreKey(key)
	if err != nil {
		return err
	}
	if err := c.store.Delete(ctx, storeKey); err != nil {
		return err
	}
	redisInfo(ctx, "wechat sdk cache deleted", log.String("key", key))
	return nil
}

func stringifyWechatSDKValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}
