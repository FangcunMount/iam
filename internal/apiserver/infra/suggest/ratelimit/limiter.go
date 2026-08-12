package ratelimit

import (
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	redis "github.com/redis/go-redis/v9"
)

// NewFromConfig 根据配置构造限流器；PerOperatorQPS<=0 时返回 nil。
// backend=redis 且 client 非空时用 Redis 固定窗口；Redis 不可用时降级 memory。
func NewFromConfig(cfg appsuggest.RateLimitConfig, redisClient *redis.Client) appsuggest.RateLimiter {
	if cfg.PerOperatorQPS <= 0 {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Backend), "redis") {
		if redisClient != nil {
			return NewRedisLimiter(redisClient, cfg)
		}
		log.Warn("suggest rate_limit.backend=redis but redis client is nil; falling back to memory limiter")
	}
	return NewMemoryLimiter(cfg)
}
