package queryprofile

import (
	"context"
	"errors"

	"github.com/FangcunMount/component-base/pkg/log"
	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

// ErrUnauthenticated 操作员未认证。
var ErrUnauthenticated = errors.New("unauthenticated")

// Service 编排权限解析 → 查询准入 → 候选召回 → 选择排序 → 输出。
type Service struct {
	cfg           Config
	scopeResolver ScopeResolver
	recaller      CandidateRecaller
	admission     domainsearch.AdmissionPolicy
	selection     domainsearch.SelectionPolicy
	disclosure    domainprofile.MobileDisclosurePolicy
	metrics       Metrics
}

// NewService 创建查询服务。
func NewService(
	cfg Config,
	scopeResolver ScopeResolver,
	recaller CandidateRecaller,
	metrics Metrics,
) *Service {
	cfg = cfg.WithDefaults()
	if metrics == nil {
		metrics = noopMetrics{}
	}
	return &Service{
		cfg:           cfg,
		scopeResolver: scopeResolver,
		recaller:      recaller,
		admission:     domainsearch.AdmissionPolicy{},
		selection:     domainsearch.SelectionPolicy{},
		disclosure:    domainprofile.MobileDisclosurePolicy{DisableMask: cfg.DisableMobileMask},
		metrics:       metrics,
	}
}

// QueryProfile 执行档案联想查询。
func (s *Service) QueryProfile(ctx context.Context, cmd Command) ([]ResultItem, error) {
	if s == nil {
		return []ResultItem{}, nil
	}
	if cmd.Principal.OperatorID <= 0 {
		return nil, ErrUnauthenticated
	}

	keyword := domainsearch.NewKeyword(cmd.Keyword)
	if keyword.String() == "" {
		return []ResultItem{}, nil
	}

	if s.scopeResolver == nil || s.recaller == nil {
		return []ResultItem{}, nil
	}

	scope, err := s.scopeResolver.ResolveScope(ctx, cmd.Principal)
	if err != nil {
		return nil, err
	}

	if keyword.IsMobileShaped() {
		log.Infow("suggest mobile-shaped keyword",
			"operator_id", cmd.Principal.OperatorID,
			"tenant_domain", cmd.Principal.TenantDomain,
			"allow_mobile_search", scope.AllowsMobileSearch(),
			"keyword_len", len([]rune(keyword.String())),
		)
	}

	decision := s.admission.Decide(keyword, scope)
	if !decision.Allowed() {
		s.metrics.RecordQuery(decision.Kind(), 0, decision.MobileShaped(keyword))
		return []ResultItem{}, nil
	}

	limit := cmd.Limit
	if limit <= 0 || limit > s.cfg.MaxResults {
		limit = s.cfg.MaxResults
	}

	candidates, err := s.recaller.Recall(ctx, RecallRequest{
		Keyword:         keyword,
		Intent:          decision.Intent(),
		CandidateBudget: s.cfg.CandidateBudget,
	})
	if err != nil {
		return nil, err
	}

	outcome := s.selection.Select(candidates, scope, keyword, limit)
	s.metrics.ObserveSelection(outcome.MatchedCount, outcome.VisibleCount)
	s.metrics.RecordQuery(decision.Kind(), len(outcome.Profiles), decision.MobileShaped(keyword))

	return toResultItems(outcome.Profiles, s.disclosure), nil
}

func toResultItems(profiles []domainprofile.SuggestibleProfile, disclosure domainprofile.MobileDisclosurePolicy) []ResultItem {
	out := make([]ResultItem, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, ResultItem{
			ProfileID:   p.ID(),
			DisplayName: p.DisplayName(),
			MobileMask:  disclosure.Disclose(p.Mobiles()),
			Weight:      p.Weight(),
		})
	}
	return out
}

var _ Querier = (*Service)(nil)
