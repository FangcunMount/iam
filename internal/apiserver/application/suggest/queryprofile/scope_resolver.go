package queryprofile

import (
	"context"

	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

// ScopeResolverService 编排授权事实、visibility 与领域 ResolutionPolicy。
type ScopeResolverService struct {
	facts      AuthorizationFactsReader
	visibility VisibilityReader
	policy     visibility.ResolutionPolicy
}

// NewScopeResolver 创建 scope resolver。
func NewScopeResolver(facts AuthorizationFactsReader, vis VisibilityReader) *ScopeResolverService {
	return &ScopeResolverService{facts: facts, visibility: vis}
}

// ResolveScope 实现 ScopeResolver。
func (r *ScopeResolverService) ResolveScope(ctx context.Context, principal visibility.Principal) (visibility.Scope, error) {
	if r == nil || r.facts == nil {
		return visibility.NewScope(false, false, 0, nil, nil), nil
	}

	authFacts, err := r.facts.ReadAuthorizationFacts(ctx, principal)
	if err != nil {
		return visibility.Scope{}, err
	}

	var visibilityIDs []int64
	if r.visibility != nil && !authFacts.PlatformListAllowed {
		ids, err := r.visibility.VisibleProfileIDs(ctx, principal)
		if err != nil {
			return visibility.Scope{}, err
		}
		visibilityIDs = ids
	}

	return r.policy.Resolve(principal, authFacts, visibilityIDs), nil
}

var _ ScopeResolver = (*ScopeResolverService)(nil)

// DecisionKindLabel 将 DecisionKind 映射为 Prometheus 策略标签。
func DecisionKindLabel(kind domainsearch.DecisionKind) string {
	switch kind {
	case domainsearch.DecisionDenied:
		return "mobile_denied"
	case domainsearch.DecisionNumericExact:
		return "numeric_exact"
	default:
		return "prefix_text"
	}
}
