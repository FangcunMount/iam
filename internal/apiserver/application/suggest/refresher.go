package suggest

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

// ProfileIndexRefresher refreshes the profile suggestion index.
type ProfileIndexRefresher struct {
	source    ProfileCandidateSource
	runtime   ProfileSuggestionRuntime
	snapshot  SnapshotWriter
	metrics   SuggestMetrics
	lastFetch time.Time
	now       func() time.Time
}

// NewProfileIndexRefresher creates a profile suggestion index refresher.
func NewProfileIndexRefresher(source ProfileCandidateSource, runtime ProfileSuggestionRuntime, snapshot SnapshotWriter, metrics SuggestMetrics) *ProfileIndexRefresher {
	if metrics == nil {
		metrics = noopSuggestMetrics{}
	}
	return &ProfileIndexRefresher{
		source:   source,
		runtime:  runtime,
		snapshot: snapshot,
		metrics:  metrics,
		now:      time.Now,
	}
}

// RunFull replaces the active index with a full candidate set.
func (r *ProfileIndexRefresher) RunFull(ctx context.Context) error {
	if r == nil || r.source == nil {
		return fmt.Errorf("suggest refresher missing source")
	}

	started := time.Now()
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
	r.lastFetch = r.now()
	r.writeSnapshot(ctx, candidates)
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
	if r.lastFetch.IsZero() {
		return nil
	}

	started := time.Now()
	defer func() {
		r.metrics.ObserveRefresh("delta", time.Since(started).Seconds())
	}()
	candidates, err := r.source.Delta(ctx, r.lastFetch)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	if r.runtime == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	if err := r.runtime.ImportDelta(candidates); err != nil {
		return err
	}
	r.lastFetch = r.now()
	r.writeSnapshot(ctx, candidates)
	log.InfoContext(ctx, "suggest delta sync completed",
		log.String("result", "success"),
		log.Int("count", len(candidates)),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return nil
}

func (r *ProfileIndexRefresher) writeSnapshot(ctx context.Context, candidates []domainsuggest.ProfileSearchTerm) {
	if r.snapshot == nil || len(candidates) == 0 {
		return
	}
	if err := r.snapshot.Write(ctx, candidates); err != nil {
		log.Warnw("suggest persist snapshot failed", "error", err)
	}
}
