package queryprofile

import (
	"context"

	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

// AuthorizationFactsReader 查询 Suggest 授权事实。
type AuthorizationFactsReader interface {
	ReadAuthorizationFacts(ctx context.Context, principal visibility.Principal) (visibility.AuthorizationFacts, error)
}

// VisibilityReader 查询可见 ProfileID 列表。
type VisibilityReader interface {
	VisibleProfileIDs(ctx context.Context, principal visibility.Principal) ([]int64, error)
}

// CandidateRecaller 按意图从索引召回候选。
type CandidateRecaller interface {
	Recall(ctx context.Context, request RecallRequest) ([]domainsearch.Candidate, error)
}

// RecallRequest 召回请求；CandidateBudget 由应用层控制。
type RecallRequest struct {
	Keyword         domainsearch.Keyword
	Intent          domainsearch.Intent
	CandidateBudget int
}

// ScopeResolver 解析可见范围。
type ScopeResolver interface {
	ResolveScope(ctx context.Context, principal visibility.Principal) (visibility.Scope, error)
}

// Metrics 查询用例指标。
type Metrics interface {
	RecordQuery(kind domainsearch.DecisionKind, resultCount int, mobileShaped bool)
	ObserveSelection(matched, visible int)
}

type noopMetrics struct{}

func (noopMetrics) RecordQuery(domainsearch.DecisionKind, int, bool) {}
func (noopMetrics) ObserveSelection(int, int)                       {}

// Querier 查询用例端口。
type Querier interface {
	QueryProfile(ctx context.Context, cmd Command) ([]ResultItem, error)
}
