package suggest

import (
	"sort"
	"strings"
)

// RankingPolicy 在去重后按权重排序并截断；同一 ProfileID 保留权重更高的项。
type RankingPolicy struct{}

// Rank 对 ProfileSearchTerm 去重、排序并 limit（不含关键词前缀加分）。
func (RankingPolicy) Rank(terms []ProfileSearchTerm, limit int) []ProfileSearchTerm {
	ranked := make([]RankedProfileSearchTerm, len(terms))
	for i, term := range terms {
		ranked[i] = RankedProfileSearchTerm{Term: term, Kind: MatchKindPrefix}
	}
	return RankingPolicy{}.RankRankedForQuery(ranked, Query{Keyword: Keyword{}, Limit: limit})
}

// RankForQuery 与 Rank 相同去重规则，并在权重相同时对 DisplayName 前缀命中关键词者优先。
func (RankingPolicy) RankForQuery(terms []ProfileSearchTerm, q Query) []ProfileSearchTerm {
	ranked := make([]RankedProfileSearchTerm, len(terms))
	for i, term := range terms {
		ranked[i] = RankedProfileSearchTerm{Term: term, Kind: MatchKindPrefix}
	}
	return RankingPolicy{}.RankRankedForQuery(ranked, q)
}

// RankRankedForQuery 去重后按 Weight、MatchKind、DisplayName 前缀、ProfileID 排序并截断。
func (RankingPolicy) RankRankedForQuery(terms []RankedProfileSearchTerm, q Query) []ProfileSearchTerm {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	prefix := strings.TrimSpace(q.Keyword.String())

	best := make(map[int64]RankedProfileSearchTerm, len(terms))
	for _, rt := range terms {
		old, exists := best[rt.Term.ProfileID]
		if !exists || rankedTermBetter(rt, old) {
			best[rt.Term.ProfileID] = rt
		}
	}
	unique := make([]RankedProfileSearchTerm, 0, len(best))
	for _, rt := range best {
		unique = append(unique, rt)
	}

	prefixBonus := func(name string) int {
		if prefix == "" {
			return 0
		}
		if strings.HasPrefix(strings.TrimSpace(name), prefix) {
			return 1
		}
		return 0
	}

	sort.SliceStable(unique, func(i, j int) bool {
		ti, tj := unique[i].Term, unique[j].Term
		if ti.Weight != tj.Weight {
			return ti.Weight > tj.Weight
		}
		pi, pj := unique[i].Kind.RankPriority(), unique[j].Kind.RankPriority()
		if pi != pj {
			return pi > pj
		}
		bi, bj := prefixBonus(ti.DisplayName), prefixBonus(tj.DisplayName)
		if bi != bj {
			return bi > bj
		}
		return ti.ProfileID < tj.ProfileID
	})

	out := make([]ProfileSearchTerm, 0, min(len(unique), limit))
	for i := 0; i < len(unique) && i < limit; i++ {
		out = append(out, unique[i].Term)
	}
	return out
}

func rankedTermBetter(a, b RankedProfileSearchTerm) bool {
	if a.Term.Weight != b.Term.Weight {
		return a.Term.Weight > b.Term.Weight
	}
	return a.Kind.RankPriority() > b.Kind.RankPriority()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
