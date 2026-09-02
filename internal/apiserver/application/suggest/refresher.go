package suggest

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

// ProfileIndexRefresher refreshes the profile suggestion index.
type ProfileIndexRefresher struct {
	source          ProfileIndexSource
	runtime         ProfileSuggestionRuntime
	metrics         SuggestMetrics
	lastFetch       time.Time
	now             func() time.Time
	refreshMu       sync.Mutex
	lastSuccessUnix atomic.Int64
}

// NewProfileIndexRefresher creates a profile suggestion index refresher.
func NewProfileIndexRefresher(source ProfileIndexSource, runtime ProfileSuggestionRuntime, metrics SuggestMetrics) *ProfileIndexRefresher {
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
func (r *ProfileIndexRefresher) RunFull(ctx context.Context) (runErr error) {
	if r == nil || r.source == nil {
		return fmt.Errorf("suggest refresher missing source")
	}
	if !r.refreshMu.TryLock() {
		r.metrics.RecordRefresh("full", "refresh_in_progress", 0, 0, time.Time{})
		return ErrRefreshInProgress
	}
	defer r.refreshMu.Unlock()

	started := time.Now()
	windowStart := r.now()
	defer func() {
		r.metrics.ObserveRefresh("full", time.Since(started).Seconds())
		if runErr != nil {
			r.metrics.RecordRefresh("full", "failed", 0, 0, time.Time{})
		}
	}()
	candidates, err := r.source.Full(ctx)
	if err != nil {
		return err
	}
	indexable := filterIndexableTerms(candidates)
	if r.runtime != nil {
		r.runtime.Replace(indexable)
	}
	r.lastFetch = windowStart
	r.lastSuccessUnix.Store(time.Now().UTC().Unix())
	upserts, tombstones := countFullItems(candidates, indexable)
	r.metrics.RecordRefresh("full", "success", upserts, tombstones, time.Now().UTC())
	log.InfoContext(ctx, "suggest full sync completed",
		log.String("result", "success"),
		log.Int("count", len(candidates)),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return nil
}

// RunDelta imports candidates changed since the previous successful refresh.
func (r *ProfileIndexRefresher) RunDelta(ctx context.Context) (runErr error) {
	if r == nil || r.source == nil {
		return fmt.Errorf("suggest refresher missing source")
	}
	if !r.refreshMu.TryLock() {
		r.metrics.RecordRefresh("delta", "refresh_in_progress", 0, 0, time.Time{})
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
		if runErr != nil {
			r.metrics.RecordRefresh("delta", "failed", 0, 0, time.Time{})
		}
	}()
	mutations, err := r.source.Delta(ctx, since)
	if err != nil {
		return err
	}
	if len(mutations) == 0 {
		r.lastFetch = windowStart
		r.lastSuccessUnix.Store(time.Now().UTC().Unix())
		r.metrics.RecordRefresh("delta", "success", 0, 0, time.Now().UTC())
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
	if err := r.runtime.ApplyDelta(mutations); err != nil {
		return err
	}
	r.lastFetch = windowStart
	r.lastSuccessUnix.Store(time.Now().UTC().Unix())
	upserts, tombstones := countMutationItems(mutations)
	r.metrics.RecordRefresh("delta", "success", upserts, tombstones, time.Now().UTC())
	log.InfoContext(ctx, "suggest delta sync completed",
		log.String("result", "success"),
		log.Int("count", len(mutations)),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return nil
}

func (r *ProfileIndexRefresher) HasSuccessfulRefresh() bool {
	return r != nil && r.lastSuccessUnix.Load() > 0
}

func filterIndexableTerms(candidates []domainsuggest.ProfileSearchTerm) []domainsuggest.ProfileSearchTerm {
	out := make([]domainsuggest.ProfileSearchTerm, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ProfileID <= 0 || candidate.DisplayName == "" {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func countFullItems(raw, indexable []domainsuggest.ProfileSearchTerm) (upserts, tombstones int) {
	upserts = len(indexable)
	tombstones = len(raw) - len(indexable)
	if tombstones < 0 {
		tombstones = 0
	}
	return upserts, tombstones
}

func countMutationItems(mutations []domainsuggest.ProfileIndexMutation) (upserts, tombstones int) {
	for _, m := range mutations {
		switch m.Operation {
		case domainsuggest.ProfileIndexUpsert:
			upserts++
		case domainsuggest.ProfileIndexDelete:
			tombstones++
		}
	}
	return upserts, tombstones
}
