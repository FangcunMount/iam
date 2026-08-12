package suggest

import (
	"context"

	cachegovernance "github.com/FangcunMount/iam/v3/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v3/internal/apiserver/cache"
	suggestratelimit "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/ratelimit"
)

// CacheFamilyInspectors 返回当前实际启用的 Suggest 限流缓存族。
func (m *SuggestModule) CacheFamilyInspectors() []cachegovernance.FamilyInspector {
	if m == nil || m.rateLimiter == nil {
		return nil
	}
	switch limiter := m.rateLimiter.(type) {
	case *suggestratelimit.RedisLimiter:
		return []cachegovernance.FamilyInspector{suggestRateLimitInspector{
			family: cachemodel.FamilySuggestRedisRateLimit,
			probe:  limiter.Ping,
		}}
	case *suggestratelimit.MemoryLimiter:
		return []cachegovernance.FamilyInspector{suggestRateLimitInspector{
			family: cachemodel.FamilySuggestMemoryRateLimit,
		}}
	default:
		return nil
	}
}

type suggestRateLimitInspector struct {
	family cachemodel.Family
	probe  func(context.Context) error
}

func (i suggestRateLimitInspector) Descriptor() cachegovernance.FamilyDescriptor {
	descriptor, _ := cachemodel.GetFamily(i.family)
	return descriptor
}

func (i suggestRateLimitInspector) Status(ctx context.Context) (cachegovernance.FamilyStatus, error) {
	status := cachegovernance.FamilyStatus{
		Family:          i.family,
		Configured:      true,
		Healthy:         true,
		EntryCountKnown: false,
	}
	if i.probe != nil {
		if err := i.probe(ctx); err != nil {
			status.Healthy = false
			return status, err
		}
	}
	return status, nil
}
