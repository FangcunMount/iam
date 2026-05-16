package suggest

import (
	"context"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

// ProfileSuggestor is the query use case exposed to transports.
type ProfileSuggestor interface {
	SuggestProfile(ctx context.Context, req SuggestProfileRequest) ([]ProfileSuggestItem, error)
}

// ProfileAccessScopeProvider resolves visible profile scope for an operating principal.
type ProfileAccessScopeProvider interface {
	ResolveProfileAccessScope(ctx context.Context, principal domainsuggest.OperatingPrincipal) (domainsuggest.ProfileAccessScope, error)
}

// ProfileCandidateSource provides profile candidates for index refreshes.
type ProfileCandidateSource interface {
	Full(ctx context.Context) ([]domainsuggest.ProfileSearchTerm, error)
	Delta(ctx context.Context, since time.Time) ([]domainsuggest.ProfileSearchTerm, error)
}

// ProfileSuggestionIndex exposes scoped suggest queries.
type ProfileSuggestionIndex interface {
	SuggestProfile(query domainsuggest.Query, scope domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm
}

// ProfileSuggestionRuntime owns the process-local suggest index lifecycle.
type ProfileSuggestionRuntime interface {
	Current() ProfileSuggestionIndex
	Replace(terms []domainsuggest.ProfileSearchTerm) ProfileSuggestionIndex
	ImportDelta(terms []domainsuggest.ProfileSearchTerm) error
}

// SnapshotWriter optionally persists refreshed candidates in an infrastructure-owned format.
type SnapshotWriter interface {
	Write(ctx context.Context, terms []domainsuggest.ProfileSearchTerm) error
}
