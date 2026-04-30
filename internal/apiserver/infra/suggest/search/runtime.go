package search

import (
	"fmt"

	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
)

// Runtime adapts the process-global suggest store to the application port.
type Runtime struct{}

// NewRuntime creates a Runtime backed by the existing process-global index.
func NewRuntime() Runtime {
	return Runtime{}
}

func (Runtime) Current() appsuggest.ProfileSuggestionIndex {
	return Current()
}

func (Runtime) Replace(candidates []domainsuggest.ProfileCandidate) appsuggest.ProfileSuggestionIndex {
	store := Load(candidates)
	Swap(store)
	return store
}

func (Runtime) ImportDelta(candidates []domainsuggest.ProfileCandidate) error {
	store := Current()
	if store == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	store.ImportCandidates(candidates)
	return nil
}
