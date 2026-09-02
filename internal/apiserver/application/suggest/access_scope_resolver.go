package suggest

import (
	"context"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

// ProfileAccessScopeResolver 编排授权事实、visibility 与领域 Scope 策略。
type ProfileAccessScopeResolver struct {
	facts      ProfileAuthorizationFactsReader
	visibility ProfileVisibilityIDsResolver
	policy     domainsuggest.ScopeResolutionPolicy
}

// NewProfileAccessScopeResolver 创建 resolver。
func NewProfileAccessScopeResolver(
	facts ProfileAuthorizationFactsReader,
	visibility ProfileVisibilityIDsResolver,
) *ProfileAccessScopeResolver {
	return &ProfileAccessScopeResolver{
		facts:      facts,
		visibility: visibility,
	}
}

// ResolveProfileAccessScope 实现 ProfileAccessScopeProvider。
func (r *ProfileAccessScopeResolver) ResolveProfileAccessScope(
	ctx context.Context,
	principal domainsuggest.OperatingPrincipal,
) (domainsuggest.ProfileAccessScope, error) {
	if r == nil || r.facts == nil {
		return domainsuggest.ProfileAccessScope{}, nil
	}

	facts, err := r.facts.ResolveAuthorizationFacts(ctx, principal)
	if err != nil {
		return domainsuggest.ProfileAccessScope{}, err
	}

	var visibilityIDs []int64
	if r.visibility != nil && !facts.PlatformListAllowed {
		ids, err := r.visibility.VisibleProfileIDs(ctx, principal)
		if err != nil {
			return domainsuggest.ProfileAccessScope{}, err
		}
		visibilityIDs = ids
	}

	return r.policy.Resolve(principal, facts, visibilityIDs), nil
}

var _ ProfileAccessScopeProvider = (*ProfileAccessScopeResolver)(nil)
