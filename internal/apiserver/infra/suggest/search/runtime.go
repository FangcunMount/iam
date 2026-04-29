package search

import (
	"fmt"

	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
)

// Runtime adapts the process-global suggest store to the application port.
type Runtime struct{}

// NewRuntime creates a Runtime backed by the existing process-global index.
func NewRuntime() Runtime {
	return Runtime{}
}

func (Runtime) Current() appsuggest.SearchIndex {
	return Current()
}

func (Runtime) Replace(lines []string) appsuggest.SearchIndex {
	store := Load(lines)
	Swap(store)
	return store
}

func (Runtime) ImportDelta(lines []string) error {
	store := Current()
	if store == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	store.ImportLines(lines)
	return nil
}
