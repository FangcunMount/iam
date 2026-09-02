package refreshindex

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
)

// Refresher 编排 Full/Delta 刷新、游标与互斥。
type Refresher struct {
	source          ProjectionSource
	writer          IndexWriter
	metrics         Metrics
	lastFetch       time.Time
	now             func() time.Time
	refreshMu       sync.Mutex
	lastSuccessUnix atomic.Int64
}

// NewRefresher 创建 refresher。
func NewRefresher(source ProjectionSource, writer IndexWriter, metrics Metrics) *Refresher {
	if metrics == nil {
		metrics = noopMetrics{}
	}
	return &Refresher{
		source:  source,
		writer:  writer,
		metrics: metrics,
		now:     time.Now,
	}
}

// RunFull 全量替换索引。
func (r *Refresher) RunFull(ctx context.Context) (runErr error) {
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
	indexable := filterIndexableProfiles(candidates)
	if r.writer != nil {
		if err := r.writer.Replace(ctx, indexable); err != nil {
			return err
		}
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

// RunDelta 增量刷新索引。
func (r *Refresher) RunDelta(ctx context.Context) (runErr error) {
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

	changes, err := r.source.Delta(ctx, since)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
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
	if r.writer == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	if err := r.writer.Apply(ctx, changes); err != nil {
		return err
	}
	r.lastFetch = windowStart
	r.lastSuccessUnix.Store(time.Now().UTC().Unix())
	upserts, tombstones := countChangeItems(changes)
	r.metrics.RecordRefresh("delta", "success", upserts, tombstones, time.Now().UTC())
	log.InfoContext(ctx, "suggest delta sync completed",
		log.String("result", "success"),
		log.Int("count", len(changes)),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return nil
}

// HasSuccessfulRefresh 是否至少成功刷新过一次。
func (r *Refresher) HasSuccessfulRefresh() bool {
	return r != nil && r.lastSuccessUnix.Load() > 0
}

func filterIndexableProfiles(candidates []domainprofile.SuggestibleProfile) []domainprofile.SuggestibleProfile {
	out := make([]domainprofile.SuggestibleProfile, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID() <= 0 || candidate.DisplayName() == "" {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func countFullItems(raw, indexable []domainprofile.SuggestibleProfile) (upserts, tombstones int) {
	upserts = len(indexable)
	tombstones = len(raw) - len(indexable)
	if tombstones < 0 {
		tombstones = 0
	}
	return upserts, tombstones
}

func countChangeItems(changes []ProjectionChange) (upserts, tombstones int) {
	for _, c := range changes {
		switch c.Kind() {
		case ChangeUpsert:
			upserts++
		case ChangeDelete:
			tombstones++
		}
	}
	return upserts, tombstones
}
