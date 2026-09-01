package jwks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// CacheFetcher 从内存缓存获取 JWKS。
type CacheFetcher struct {
	mu              sync.RWMutex
	keySet          jwk.Set
	updated         time.Time
	ttl             time.Duration
	maxStale        time.Duration
	fallbackOnError bool
	next            KeyFetcher
	stats           *FetcherStats
}

// CacheFetcherOption 缓存 Fetcher 配置选项。
type CacheFetcherOption func(*CacheFetcher)

// WithCacheTTL 设置缓存 TTL。
func WithCacheTTL(ttl time.Duration) CacheFetcherOption {
	return func(f *CacheFetcher) {
		f.ttl = ttl
	}
}

// WithCacheMaxStale 设置旧缓存允许继续使用的最长时间。
func WithCacheMaxStale(maxStale time.Duration) CacheFetcherOption {
	return func(f *CacheFetcher) {
		f.maxStale = maxStale
	}
}

// WithCacheFallbackOnError 控制刷新失败时是否允许使用未超过 maxStale 的旧缓存。
func WithCacheFallbackOnError(enabled bool) CacheFetcherOption {
	return func(f *CacheFetcher) {
		f.fallbackOnError = enabled
	}
}

// WithCacheNext 设置下一个 fetcher。
func WithCacheNext(next KeyFetcher) CacheFetcherOption {
	return func(f *CacheFetcher) {
		f.next = next
	}
}

// NewCacheFetcher 创建缓存 Fetcher。
func NewCacheFetcher(opts ...CacheFetcherOption) *CacheFetcher {
	f := &CacheFetcher{
		ttl:   5 * time.Minute,
		stats: &FetcherStats{},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *CacheFetcher) Name() string {
	return "cache"
}

func (f *CacheFetcher) Fetch(ctx context.Context) (jwk.Set, error) {
	f.stats.IncrAttempts()

	keySet, updated := f.cachedSnapshot()
	if keySet != nil && time.Since(updated) < f.ttl {
		f.stats.IncrSuccesses()
		return keySet, nil
	}

	if f.next == nil {
		f.stats.IncrFailures()
		return nil, fmt.Errorf("cache: no data and no next fetcher")
	}

	return f.fetchAndRefresh(ctx, keySet, updated)
}

// Update 手动更新缓存。
func (f *CacheFetcher) Update(keySet jwk.Set) {
	f.mu.Lock()
	f.keySet = keySet
	f.updated = time.Now()
	f.mu.Unlock()
}

// Stats 返回统计信息。
func (f *CacheFetcher) Stats() FetcherStats {
	return f.stats.Snapshot()
}
