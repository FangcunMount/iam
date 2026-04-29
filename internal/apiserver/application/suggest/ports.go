package suggest

import domainsuggest "github.com/FangcunMount/iam/internal/apiserver/domain/suggest"

// SearchIndex exposes the query behavior required by the suggest application.
type SearchIndex interface {
	Suggest(keyword string, max int, pad int) []domainsuggest.Term
}

// IndexRuntime owns the process-local suggest index lifecycle.
type IndexRuntime interface {
	Current() SearchIndex
	Replace(lines []string) SearchIndex
	ImportDelta(lines []string) error
}
