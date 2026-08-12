package suggest

import (
	"context"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

// ProfileSuggestor is the query use case exposed to transports.
type ProfileSuggestor interface {
	SuggestProfile(ctx context.Context, req SuggestProfileRequest) ([]ProfileSuggestItem, error)
}

// ProfileAccessScopeProvider resolves visible profile scope for an operating principal.
type ProfileAccessScopeProvider interface {
	ResolveProfileAccessScope(ctx context.Context, principal domainsuggest.OperatingPrincipal) (domainsuggest.ProfileAccessScope, error)
}

// ProfileVisibilityIDsResolver 可选：为复杂数据权限补充可见 ProfileID（预计算、缓存等），结果并入 ProfileAccessScope.ProfileIDs。
type ProfileVisibilityIDsResolver interface {
	VisibleProfileIDs(ctx context.Context, principal domainsuggest.OperatingPrincipal) ([]int64, error)
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

// RateLimiter 按 operator 限流；nil 表示关闭（由 infra 实现）。
type RateLimiter interface {
	Allow(operatorID int64, mobileKeyword bool) bool
}
