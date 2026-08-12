package access

import (
	"context"
	"sync"
	"time"

	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

type visibilityCacheEntry struct {
	ids       []int64
	err       error
	expiresAt time.Time
}

// CachedProfileVisibilityResolver 为 ProfileVisibilityIDsResolver 增加按 operator 的短 TTL 缓存。
type CachedProfileVisibilityResolver struct {
	inner appsuggest.ProfileVisibilityIDsResolver
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[int64]visibilityCacheEntry
}

// NewCachedProfileVisibilityResolver 包装 inner；ttl<=0 时直接返回 inner。
func NewCachedProfileVisibilityResolver(inner appsuggest.ProfileVisibilityIDsResolver, ttl time.Duration) appsuggest.ProfileVisibilityIDsResolver {
	if inner == nil || ttl <= 0 {
		return inner
	}
	return &CachedProfileVisibilityResolver{
		inner:   inner,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[int64]visibilityCacheEntry),
	}
}

// VisibleProfileIDs 实现 ProfileVisibilityIDsResolver。
func (c *CachedProfileVisibilityResolver) VisibleProfileIDs(ctx context.Context, principal domainsuggest.OperatingPrincipal) ([]int64, error) {
	if c == nil || c.inner == nil || principal.OperatorID <= 0 {
		return nil, nil
	}
	now := c.now()

	c.mu.Lock()
	if ent, ok := c.entries[principal.OperatorID]; ok && now.Before(ent.expiresAt) {
		c.mu.Unlock()
		if ent.err != nil {
			return nil, ent.err
		}
		return append([]int64(nil), ent.ids...), nil
	}
	c.mu.Unlock()

	ids, err := c.inner.VisibleProfileIDs(ctx, principal)
	c.mu.Lock()
	c.entries[principal.OperatorID] = visibilityCacheEntry{
		ids:       append([]int64(nil), ids...),
		err:       err,
		expiresAt: now.Add(c.ttl),
	}
	c.mu.Unlock()
	return ids, err
}
