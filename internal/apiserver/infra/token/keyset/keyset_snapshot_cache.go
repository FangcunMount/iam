package keyset

import (
	"sync"
	"time"
)

type keySetSnapshotCache struct {
	mu      sync.RWMutex
	jwks    *JWKS
	tag     CacheTag
	builtAt time.Time
	ttl     time.Duration
	now     func() time.Time
}

func newKeySetSnapshotCache(ttl time.Duration, now func() time.Time) *keySetSnapshotCache {
	if ttl <= 0 {
		ttl = time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &keySetSnapshotCache{
		ttl: ttl,
		now: now,
	}
}

func (c *keySetSnapshotCache) store(jwksObj JWKS, tag CacheTag) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	copied := JWKS{Keys: append([]PublicJWK(nil), jwksObj.Keys...)}
	c.jwks = &copied
	c.tag = tag
	c.builtAt = c.now()
}

func (c *keySetSnapshotCache) currentTag() (CacheTag, bool) {
	if c == nil {
		return CacheTag{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.tag.ETag == "" || c.builtAt.IsZero() {
		return CacheTag{}, false
	}
	if c.now().Sub(c.builtAt) >= c.ttl {
		return CacheTag{}, false
	}
	return c.tag, true
}

func (c *keySetSnapshotCache) status() SnapshotStatus {
	if c == nil {
		return SnapshotStatus{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := SnapshotStatus{
		Cached:   c.jwks != nil,
		CacheTag: c.tag,
	}
	if c.jwks != nil {
		status.KeyCount = len(c.jwks.Keys)
	}
	if !c.builtAt.IsZero() {
		builtAt := c.builtAt
		status.LastBuildTime = &builtAt
	}
	return status
}

func (c *keySetSnapshotCache) nowUTC() time.Time {
	if c == nil || c.now == nil {
		return time.Now().UTC()
	}
	return c.now().UTC()
}
