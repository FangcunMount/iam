package suggest

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/log"
	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

// Service 提供 suggest 查询
type Service struct {
	cfg           Config
	runtime       ProfileSuggestionRuntime
	scopeProvider ProfileAccessScopeProvider
	searchPolicy  domainsuggest.SearchPolicy
	metrics       SuggestMetrics
}

// NewServiceWithRuntime creates a suggest service with an explicit index runtime.
func NewServiceWithRuntime(cfg Config, runtime ProfileSuggestionRuntime, scope ProfileAccessScopeProvider, metrics SuggestMetrics) *Service {
	cfg = cfg.WithDefaults()
	if metrics == nil {
		metrics = noopSuggestMetrics{}
	}
	return &Service{
		cfg:           cfg,
		runtime:       runtime,
		scopeProvider: scope,
		searchPolicy:  domainsuggest.SearchPolicy{},
		metrics:       metrics,
	}
}

// SuggestProfile 按当前操作员可见范围查询档案联想。
func (s *Service) SuggestProfile(ctx context.Context, req SuggestProfileRequest) ([]ProfileSuggestItem, error) {
	if s == nil || s.runtime == nil {
		return []ProfileSuggestItem{}, nil
	}
	if s.scopeProvider == nil {
		return []ProfileSuggestItem{}, nil
	}
	if req.Principal.OperatorID <= 0 {
		return nil, ErrUnauthenticated
	}

	keyword := domainsuggest.NewKeyword(req.Keyword)
	if keyword.String() == "" {
		return []ProfileSuggestItem{}, nil
	}

	scope, err := s.scopeProvider.ResolveProfileAccessScope(ctx, req.Principal)
	if err != nil {
		return nil, err
	}

	if keyword.IsMobileShaped() {
		log.Infow("suggest mobile-shaped keyword",
			"operator_id", req.Principal.OperatorID,
			"tenant_domain", req.Principal.TenantDomain,
			"allow_mobile_search", scope.AllowMobileSearch,
			"keyword_len", len([]rune(keyword.String())),
		)
	}

	decision := s.searchPolicy.Decide(keyword, scope)

	index := s.runtime.Current()
	if index == nil {
		return []ProfileSuggestItem{}, nil
	}

	limit := req.Limit
	if limit <= 0 || limit > s.cfg.MaxResults {
		limit = s.cfg.MaxResults
	}

	query := domainsuggest.NewQueryWithMode(
		req.Keyword, decision.Mode(), limit, s.cfg.InternalMaxResults, s.cfg.KeyPadLen, s.cfg.WildcardKeyCap,
	)

	var terms []domainsuggest.ProfileSearchTerm
	if decision.Allowed() {
		terms = index.SuggestProfile(query, scope)
	}

	s.metrics.RecordQuery(decision.MetricName(), len(terms), decision.MobileShaped(keyword))
	return toProfileSuggestItems(terms, s.cfg.DisableMobileMask), nil
}

func toProfileSuggestItems(terms []domainsuggest.ProfileSearchTerm, disableMask bool) []ProfileSuggestItem {
	out := make([]ProfileSuggestItem, 0, len(terms))
	for _, t := range terms {
		mask := domainsuggest.MaskMobiles(t.Mobiles)
		if disableMask {
			if len(t.Mobiles) > 0 {
				mask = t.Mobiles[0]
			} else {
				mask = ""
			}
		}
		out = append(out, ProfileSuggestItem{
			ProfileID:   t.ProfileID,
			DisplayName: t.DisplayName,
			MobileMask:  mask,
			Weight:      t.Weight,
		})
	}
	return out
}
