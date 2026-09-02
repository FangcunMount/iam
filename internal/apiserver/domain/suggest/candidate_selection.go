package suggest

// CandidateSelectionResult 携带最终候选项与 scope 过滤后的可见数（排序前）。
type CandidateSelectionResult struct {
	Terms        []ProfileSearchTerm
	MatchedCount int
	VisibleCount int
}

// CandidateSelectionPolicy 固定 compile → filter → deduplicate → rank → limit。
type CandidateSelectionPolicy struct {
	scope   ScopePolicy
	ranking RankingPolicy
}

// NewCandidateSelectionPolicy 创建候选选择策略。
func NewCandidateSelectionPolicy() CandidateSelectionPolicy {
	return CandidateSelectionPolicy{}
}

// Select 在 scope 内去重、排序并截断。
func (p CandidateSelectionPolicy) Select(
	candidates []RankedProfileSearchTerm,
	scope ProfileAccessScope,
	query Query,
) CandidateSelectionResult {
	matched := len(candidates)
	if matched == 0 {
		return CandidateSelectionResult{Terms: nil, MatchedCount: 0, VisibleCount: 0}
	}

	scopePolicy := p.scope
	if scopePolicy == (ScopePolicy{}) {
		scopePolicy = ScopePolicy{}
	}
	ranking := p.ranking
	if ranking == (RankingPolicy{}) {
		ranking = RankingPolicy{}
	}

	compiled := CompileProfileAccessScope(scope)
	visible := make([]RankedProfileSearchTerm, 0, min(len(candidates), query.Limit))
	for _, rt := range candidates {
		if !scopePolicy.AllowsCompiled(compiled, rt.Term) {
			continue
		}
		visible = append(visible, rt)
	}

	terms := ranking.RankRankedForQuery(visible, query)
	return CandidateSelectionResult{
		Terms:        terms,
		MatchedCount: matched,
		VisibleCount: len(visible),
	}
}
