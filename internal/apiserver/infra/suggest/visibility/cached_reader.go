package visibility

import (
	"context"
	"sync"
	"time"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	domainvisibility "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

type cacheEntry struct {
	ids       []int64
	err       error
	expiresAt time.Time
}

// CachedReader 为 VisibilityReader 增加按 operator 的短 TTL 缓存。
type CachedReader struct {
	inner appquery.VisibilityReader
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[int64]cacheEntry
}

// NewCachedReader 包装 inner；ttl<=0 时直接返回 inner。
func NewCachedReader(inner appquery.VisibilityReader, ttl time.Duration) appquery.VisibilityReader {
	if inner == nil || ttl <= 0 {
		return inner
	}
	return &CachedReader{
		inner:   inner,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[int64]cacheEntry),
	}
}

// VisibleProfileIDs 实现 VisibilityReader。
func (c *CachedReader) VisibleProfileIDs(ctx context.Context, principal domainvisibility.Principal) ([]int64, error) {
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
	c.entries[principal.OperatorID] = cacheEntry{
		ids:       append([]int64(nil), ids...),
		err:       err,
		expiresAt: now.Add(c.ttl),
	}
	c.mu.Unlock()
	return ids, err
}

var _ appquery.VisibilityReader = (*CachedReader)(nil)
