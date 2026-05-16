package suggest

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/log"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	suggestmetrics "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/metrics"
)

// Service 提供 suggest 查询
type Service struct {
	cfg           Config
	runtime       ProfileSuggestionRuntime
	scopeProvider ProfileAccessScopeProvider
	strategies    []ProfileSearchStrategy
}

// NewService 创建（无运行时索引）Service。
func NewService(cfg Config) *Service {
	return NewServiceWithRuntime(cfg, nil, nil)
}

// NewServiceWithRuntime creates a suggest service with an explicit index runtime.
func NewServiceWithRuntime(cfg Config, runtime ProfileSuggestionRuntime, scope ProfileAccessScopeProvider) *Service {
	return NewServiceWithRuntimeStrategies(cfg, runtime, scope, nil)
}

// NewServiceWithRuntimeStrategies 与 NewServiceWithRuntime 相同，但可注入自定义策略链；strategies 为 nil 时使用 DefaultProfileSearchStrategies。
func NewServiceWithRuntimeStrategies(cfg Config, runtime ProfileSuggestionRuntime, scope ProfileAccessScopeProvider, strategies []ProfileSearchStrategy) *Service {
	cfg = cfg.WithDefaults()
	strat := strategies
	if len(strat) == 0 {
		strat = DefaultProfileSearchStrategies()
	}
	out := make([]ProfileSearchStrategy, 0, len(strat))
	for _, s := range strat {
		if s != nil {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		out = DefaultProfileSearchStrategies()
	}
	return &Service{cfg: cfg, runtime: runtime, scopeProvider: scope, strategies: out}
}

// SuggestProfile 按当前操作员可见范围查询档案联想。
func (s *Service) SuggestProfile(ctx context.Context, req SuggestProfileRequest) ([]ProfileSuggestItem, error) {
	if s == nil || s.runtime == nil {
		return []ProfileSuggestItem{}, nil
	}
	if s.scopeProvider == nil {
		return []ProfileSuggestItem{}, nil
	}
	if req.Principal.OperatorID <= 0 && !req.Principal.IsSuperAdmin {
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

	if keyword.IsDigits() && domainsuggest.LooksLikeMobile(keyword.String()) {
		log.Infow("suggest mobile-shaped keyword",
			"operator_id", req.Principal.OperatorID,
			"tenant_domain", req.Principal.TenantDomain,
			"allow_mobile_search", scope.AllowMobileSearch,
			"keyword_len", len([]rune(keyword.String())),
		)
	}

	index := s.runtime.Current()
	if index == nil {
		return []ProfileSuggestItem{}, nil
	}

	limit := req.Limit
	if limit <= 0 || limit > s.cfg.MaxResults {
		limit = s.cfg.MaxResults
	}

	query := domainsuggest.NewQuery(req.Keyword, limit, s.cfg.InternalMaxResults, s.cfg.KeyPadLen, s.cfg.TrieWildcardKeyCap)
	strategy := selectProfileSearchStrategy(s.strategies, keyword, scope)
	if strategy == nil {
		return []ProfileSuggestItem{}, nil
	}
	terms := strategy.Search(ctx, index, query, scope)
	mobile := keyword.IsDigits() && domainsuggest.LooksLikeMobile(keyword.String())
	strategyName := "none"
	if strategy != nil {
		strategyName = strategy.Name()
	}
	suggestmetrics.RecordQuery(strategyName, len(terms), mobile)
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
