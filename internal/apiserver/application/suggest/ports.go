package suggest

import (
	"context"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

// ProfileSuggestor is the query use case exposed to transports.
type ProfileSuggestor interface {
	Suggest(ctx context.Context, keyword string) []domainsuggest.Term
}

// ProfileCandidateSource provides profile candidates for index refreshes.
type ProfileCandidateSource interface {
	Full(ctx context.Context) ([]domainsuggest.ProfileCandidate, error)
	Delta(ctx context.Context, since time.Time) ([]domainsuggest.ProfileCandidate, error)
}

// ProfileSuggestionIndex exposes the query behavior required by the suggest application.
type ProfileSuggestionIndex interface {
	Suggest(query domainsuggest.Query) []domainsuggest.Term
}

// ProfileSuggestionRuntime owns the process-local suggest index lifecycle.
type ProfileSuggestionRuntime interface {
	Current() ProfileSuggestionIndex
	Replace(candidates []domainsuggest.ProfileCandidate) ProfileSuggestionIndex
	ImportDelta(candidates []domainsuggest.ProfileCandidate) error
}

// SnapshotWriter optionally persists refreshed candidates in an infrastructure-owned format.
type SnapshotWriter interface {
	Write(ctx context.Context, candidates []domainsuggest.ProfileCandidate) error
}
