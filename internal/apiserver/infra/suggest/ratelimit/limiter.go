package ratelimit

import (
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
	redis "github.com/redis/go-redis/v9"
)

// Limiter 按 operator 限流。
type Limiter interface {
	Allow(operatorID int64, mobileKeyword bool) bool
}

// NewFromConfig 根据配置构造限流器；PerOperatorQPS<=0 时返回 nil。
func NewFromConfig(cfg Config, redisClient *redis.Client) Limiter {
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
