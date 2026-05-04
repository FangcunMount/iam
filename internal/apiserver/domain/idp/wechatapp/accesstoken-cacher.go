package wechatapp

import (
	"context"
	"errors"
	"time"

	cacheflow "github.com/FangcunMount/iam/v2/internal/apiserver/cache"
)

type accessTokenCacher struct {
	cache    AccessTokenCache
	provider AppTokenProvider
	// 策略
	refreshSkew time.Duration // 提前刷新窗口，e.g., 120s
	cacheTTLMin time.Duration // 最小缓存TTL保护，避免抖动
}

// 确保 accessTokenCacher 实现了相应的接口
var _ AccessTokenCacher = (*accessTokenCacher)(nil)

// NewAccessTokenCacher 创建访问令牌缓存器实例
func NewAccessTokenCacher(cache AccessTokenCache, provider AppTokenProvider) AccessTokenCacher {
	return &accessTokenCacher{
		cache:       cache,
		provider:    provider,
		refreshSkew: 120 * time.Second,
		cacheTTLMin: 60 * time.Second,
	}
}

// EnsureToken 单飞刷新 + 过期缓冲 获取访问令牌
func (s *accessTokenCacher) EnsureToken(ctx context.Context, app *WechatApp, skew time.Duration) (string, error) {
	if app == nil {
		return "", errors.New("nil app")
	}
	if skew <= 0 {
		skew = s.refreshSkew
	}

	token, err := cacheflow.LockedReadThrough[AppAccessToken](ctx, cacheflow.LockedReadThroughOptions[AppAccessToken]{
		ReadThroughOptions: cacheflow.ReadThroughOptions[AppAccessToken]{
			Get: func(ctx context.Context) (*AppAccessToken, error) {
				return s.cache.Get(ctx, app.AppID)
			},
			Valid: func(aat *AppAccessToken) bool {
				return aat != nil && aat.IsValid(time.Now(), skew)
			},
			Load: func(ctx context.Context) (*AppAccessToken, error) {
				return s.provider.Fetch(ctx, app)
			},
			TTL: func(aat *AppAccessToken) time.Duration {
				ttl := time.Until(aat.ExpiresAt) - skew
				if ttl < s.cacheTTLMin {
					ttl = s.cacheTTLMin
				}
				return ttl
			},
			Set: func(ctx context.Context, aat *AppAccessToken, ttl time.Duration) error {
				return s.cache.Set(ctx, app.AppID, aat, ttl)
			},
			IgnoreGetError: true,
		},
		Lock: func(ctx context.Context) (bool, func(), error) {
			return s.cache.TryLockRefresh(ctx, app.AppID, 10*time.Second)
		},
		RereadUsable: func(aat *AppAccessToken) bool {
			return aat != nil && aat.Token != ""
		},
		LockMissError: errors.New("access_token refresh in progress, please retry"),
	})
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", errors.New("access_token refresh in progress, please retry")
	}
	return token.Token, nil
}
