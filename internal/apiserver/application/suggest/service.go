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
	strategies    []ProfileSearchStrategy
	metrics       SuggestMetrics
}

// NewServiceWithRuntime creates a suggest service with an explicit index runtime.
func NewServiceWithRuntime(cfg Config, runtime ProfileSuggestionRuntime, scope ProfileAccessScopeProvider, metrics SuggestMetrics) *Service {
	return NewServiceWithRuntimeStrategies(cfg, runtime, scope, nil, metrics)
}

// NewServiceWithRuntimeStrategies 与 NewServiceWithRuntime 相同，但可注入自定义策略链；strategies 为 nil 时使用 DefaultProfileSearchStrategies。
func NewServiceWithRuntimeStrategies(cfg Config, runtime ProfileSuggestionRuntime, scope ProfileAccessScopeProvider, strategies []ProfileSearchStrategy, metrics SuggestMetrics) *Service {
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
	if metrics == nil {
		metrics = noopSuggestMetrics{}
	}
	return &Service{cfg: cfg, runtime: runtime, scopeProvider: scope, strategies: out, metrics: metrics}
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

	// 1. 构建关键词
	keyword := domainsuggest.NewKeyword(req.Keyword)
	if keyword.String() == "" {
		return []ProfileSuggestItem{}, nil
	}

	// 2. 构建权限范围
	scope, err := s.scopeProvider.ResolveProfileAccessScope(ctx, req.Principal)
	if err != nil {
		return nil, err
	}

	// 3. 日志记录
	if keyword.IsDigits() && domainsuggest.LooksLikeMobile(keyword.String()) {
		log.Infow("suggest mobile-shaped keyword",
			"operator_id", req.Principal.OperatorID,
			"tenant_domain", req.Principal.TenantDomain,
			"allow_mobile_search", scope.AllowMobileSearch,
			"keyword_len", len([]rune(keyword.String())),
		)
	}

	// 4. 获取索引
	index := s.runtime.Current()
	if index == nil {
		return []ProfileSuggestItem{}, nil
	}

	// 5. 限制返回结果数量
	limit := req.Limit
	if limit <= 0 || limit > s.cfg.MaxResults {
		limit = s.cfg.MaxResults
	}

	// 6. 构建查询
	query := domainsuggest.NewQuery(req.Keyword, limit, s.cfg.InternalMaxResults, s.cfg.KeyPadLen, s.cfg.WildcardKeyCap)

	// 7. 选择搜索策略
	strategy := selectProfileSearchStrategy(s.strategies, keyword, scope)
	if strategy == nil {
		return []ProfileSuggestItem{}, nil
	}

	// 8. 执行搜索
	terms := strategy.Search(ctx, index, query, scope)

	// 9. 记录指标
	s.metrics.RecordQuery(strategy.Name(), len(terms), keyword.IsDigits() && domainsuggest.LooksLikeMobile(keyword.String()))
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
