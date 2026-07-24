package suggest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

// ProfileIndexRefresher refreshes the profile suggestion index.
type ProfileIndexRefresher struct {
	source    ProfileCandidateSource
	runtime   ProfileSuggestionRuntime
	metrics   SuggestMetrics
	lastFetch time.Time
	now       func() time.Time
	refreshMu sync.Mutex
}

// NewProfileIndexRefresher creates a profile suggestion index refresher.
func NewProfileIndexRefresher(source ProfileCandidateSource, runtime ProfileSuggestionRuntime, metrics SuggestMetrics) *ProfileIndexRefresher {
	if metrics == nil {
		metrics = noopSuggestMetrics{}
	}
	return &ProfileIndexRefresher{
		source:  source,
		runtime: runtime,
		metrics: metrics,
		now:     time.Now,
	}
}

// RunFull replaces the active index with a full candidate set.
func (r *ProfileIndexRefresher) RunFull(ctx context.Context) error {
	if r == nil || r.source == nil {
		return fmt.Errorf("suggest refresher missing source")
	}
	if !r.refreshMu.TryLock() {
		return ErrRefreshInProgress
	}
	defer r.refreshMu.Unlock()

	started := time.Now()
	windowStart := r.now()
	defer func() {
		r.metrics.ObserveRefresh("full", time.Since(started).Seconds())
	}()
	candidates, err := r.source.Full(ctx)
	if err != nil {
		return err
	}
	if r.runtime != nil {
		r.runtime.Replace(candidates)
	}
	r.lastFetch = windowStart
	log.InfoContext(ctx, "suggest full sync completed",
		log.String("result", "success"),
		log.Int("count", len(candidates)),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return nil
}

// RunDelta imports candidates changed since the previous successful refresh.
func (r *ProfileIndexRefresher) RunDelta(ctx context.Context) error {
	if r == nil || r.source == nil {
		return fmt.Errorf("suggest refresher missing source")
	}
	if !r.refreshMu.TryLock() {
		return ErrRefreshInProgress
	}
	defer r.refreshMu.Unlock()
	if r.lastFetch.IsZero() {
		return nil
	}

	started := time.Now()
	since := r.lastFetch
	windowStart := r.now()
	defer func() {
		r.metrics.ObserveRefresh("delta", time.Since(started).Seconds())
	}()
	candidates, err := r.source.Delta(ctx, since)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		r.lastFetch = windowStart
		log.InfoContext(ctx, "suggest delta sync completed",
			log.String("result", "success"),
			log.Int("count", 0),
			log.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
		return nil
	}
	if r.runtime == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	if err := r.runtime.ImportDelta(candidates); err != nil {
		return err
	}
	r.lastFetch = windowStart
	log.InfoContext(ctx, "suggest delta sync completed",
		log.String("result", "success"),
		log.Int("count", len(candidates)),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return nil
}
