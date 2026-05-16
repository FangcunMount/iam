package search

import (
	"fmt"
	"sync/atomic"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	suggestmetrics "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/metrics"
)

// Runtime 持有进程内 suggest Store 快照指针。
type Runtime struct {
	active atomic.Value // *Store
}

// NewRuntime creates a Runtime with empty active store (Current may yield empty store)。
func NewRuntime() *Runtime {
	r := &Runtime{}
	r.active.Store((*Store)(nil))
	return r
}

// Current implements ProfileSuggestionRuntime.
func (r *Runtime) Current() appsuggest.ProfileSuggestionIndex {
	if r == nil {
		return nil
	}
	v := r.active.Load()
	if v == nil {
		return nil
	}
	return v.(*Store)
}

// Replace replaces the active index.
func (r *Runtime) Replace(terms []domainsuggest.ProfileSearchTerm) appsuggest.ProfileSuggestionIndex {
	if r == nil {
		return nil
	}
	store := Load(terms)
	r.active.Store(store)
	suggestmetrics.SetIndexTerms(store.Len())
	return store
}

// ImportDelta merges terms into the active store。
func (r *Runtime) ImportDelta(terms []domainsuggest.ProfileSearchTerm) error {
	if r == nil {
		return fmt.Errorf("suggest runtime is nil")
	}
	cur := r.Current()
	if cur == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	store, ok := cur.(*Store)
	if !ok || store == nil {
		return fmt.Errorf("invalid suggest store")
	}
	store.ImportTerms(terms)
	suggestmetrics.SetIndexTerms(store.Len())
	return nil
}
