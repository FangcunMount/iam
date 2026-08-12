package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	redis "github.com/redis/go-redis/v9"
)

const (
	redisWindowSeconds = 1
	redisRateLimitLua  = `
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
if n <= tonumber(ARGV[2]) then
  return 1
end
return 0
`
)

// RedisLimiter 使用 Redis 固定窗口（约 1s）计数，多副本共享配额。
type RedisLimiter struct {
	client *redis.Client

	stdMax int
	mobMax int
}

// Ping 验证限流器所用 Redis 后端可用。
func (r *RedisLimiter) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("suggest Redis rate limiter unavailable")
	}
	return r.client.Ping(ctx).Err()
}

// NewRedisLimiter 创建 Redis 限流器。
func NewRedisLimiter(client *redis.Client, cfg appsuggest.RateLimitConfig) *RedisLimiter {
	stdBurst := cfg.PerOperatorBurst
	if stdBurst <= 0 {
		stdBurst = 5
	}
	mobBurst := cfg.MobileKeywordPerOperatorBurst
	if mobBurst <= 0 {
		mobBurst = stdBurst
	}
	return &RedisLimiter{
		client: client,
		stdMax: stdBurst,
		mobMax: mobBurst,
	}
}

// Allow 对 operatorID 记账一次；Redis 错误时 fail-open 并打 warn。
func (r *RedisLimiter) Allow(operatorID int64, mobileKeyword bool) bool {
	if r == nil || r.client == nil || operatorID <= 0 {
		return true
	}
	kind := "std"
	max := r.stdMax
	if mobileKeyword {
		kind = "mob"
		max = r.mobMax
	}
	if max <= 0 {
		return true
	}
	key := fmt.Sprintf("iam:suggest:rl:%s:%d", kind, operatorID)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res, err := r.client.Eval(ctx, redisRateLimitLua, []string{key}, redisWindowSeconds, max).Int()
	if err != nil {
		log.Warnw("suggest redis rate limiter failed, allowing request", "error", err, "operator_id", operatorID)
		return true
	}
	return res == 1
}
