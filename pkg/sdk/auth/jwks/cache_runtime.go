package jwks

import (
	"context"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

func (f *CacheFetcher) cachedSnapshot() (jwk.Set, time.Time) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.keySet, f.updated
}

func (f *CacheFetcher) fetchAndRefresh(ctx context.Context, stale jwk.Set, staleUpdated time.Time) (jwk.Set, error) {
	newKeySet, err := f.next.Fetch(ctx)
	if err != nil {
		if f.canUseStale(stale, staleUpdated) {
			f.stats.IncrSuccesses()
			return stale, nil
		}
		f.stats.IncrFailures()
		return nil, err
	}

	f.mu.Lock()
	f.keySet = newKeySet
	f.updated = time.Now()
	f.mu.Unlock()

	f.stats.IncrSuccesses()
	return newKeySet, nil
}

func (f *CacheFetcher) canUseStale(stale jwk.Set, updated time.Time) bool {
	if !f.fallbackOnError || stale == nil || updated.IsZero() || f.maxStale <= 0 {
		return false
	}
	return time.Since(updated) <= f.maxStale
}
