package memory

import (
	"context"
	"fmt"
	"sync/atomic"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

// Runtime 持有进程内 suggest Store 快照指针。
type Runtime struct {
	active  atomic.Value // *Store
	cfg     Config
	metrics IndexMetrics
}

// NewRuntime 创建 Runtime。
func NewRuntime(cfg Config, metrics IndexMetrics) *Runtime {
	if metrics == nil {
		metrics = noopIndexMetrics{}
	}
	r := &Runtime{cfg: cfg.WithDefaults(), metrics: metrics}
	r.active.Store((*Store)(nil))
	return r
}

// CurrentStore 返回当前 Store；nil 表示未初始化。
func (r *Runtime) CurrentStore() *Store {
	if r == nil {
		return nil
	}
	v := r.active.Load()
	if v == nil {
		return nil
	}
	return v.(*Store)
}

// Recall 实现 CandidateRecaller；未初始化时返回空。
func (r *Runtime) Recall(ctx context.Context, request appquery.RecallRequest) ([]domainsearch.Candidate, error) {
	store := r.CurrentStore()
	if store == nil {
		return []domainsearch.Candidate{}, nil
	}
	return store.Recall(ctx, request)
}

// Replace 全量替换索引。
func (r *Runtime) Replace(ctx context.Context, profiles []domainprofile.SuggestibleProfile) error {
	if r == nil {
		return fmt.Errorf("suggest runtime is nil")
	}
	store := Load(profiles, r.cfg)
	r.active.Store(store)
	r.metrics.SetIndexTerms(store.Len())
	return nil
}

// Apply 应用增量变更。
func (r *Runtime) Apply(ctx context.Context, changes []apprefresh.ProjectionChange) error {
	if r == nil {
		return fmt.Errorf("suggest runtime is nil")
	}
	store := r.CurrentStore()
	if store == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	store.ApplyChanges(changes)
	r.metrics.SetIndexTerms(store.Len())
	return nil
}

var (
	_ appquery.CandidateRecaller = (*Runtime)(nil)
	_ apprefresh.IndexWriter     = (*Runtime)(nil)
)
