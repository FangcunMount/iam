package suggest

import (
	redis "github.com/redis/go-redis/v9"

	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
)

// CollectREST wires suggest REST collaborators when the module is available.
func CollectREST(available bool, mod *SuggestModule, deps *resttransport.Deps, redisClient *redis.Client) {
	if !available || mod == nil || deps == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	deps.Suggest.Service = caps.Service
	deps.Suggest.RateLimit = caps.RateLimit
	deps.Suggest.Metrics = caps.Metrics
	deps.Suggest.RateLimiter = caps.RateLimiter
	deps.Suggest.RedisClient = redisClient
}
