package suggest

import (
	"sort"
	"strings"
)

// RankingPolicy 在去重后按权重排序并截断；同一 ProfileID 保留权重更高的项。
type RankingPolicy struct{}

// Rank 对 ProfileSearchTerm 去重、排序并 limit（不含关键词前缀加分）。
func (RankingPolicy) Rank(terms []ProfileSearchTerm, limit int) []ProfileSearchTerm {
	return RankingPolicy{}.RankForQuery(terms, Query{Keyword: Keyword{}, Limit: limit})
}

// RankForQuery 与 Rank 相同去重规则，并在权重相同时对 DisplayName 前缀命中关键词者优先。
func (RankingPolicy) RankForQuery(terms []ProfileSearchTerm, q Query) []ProfileSearchTerm {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	prefix := strings.TrimSpace(q.Keyword.String())

	best := make(map[int64]ProfileSearchTerm, len(terms))
	for _, term := range terms {
		old, exists := best[term.ProfileID]
		if !exists || term.Weight > old.Weight {
			best[term.ProfileID] = term
		}
	}
	unique := make([]ProfileSearchTerm, 0, len(best))
	for _, term := range best {
		unique = append(unique, term)
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
		if unique[i].Weight != unique[j].Weight {
			return unique[i].Weight > unique[j].Weight
		}
		pi, pj := prefixBonus(unique[i].DisplayName), prefixBonus(unique[j].DisplayName)
		if pi != pj {
			return pi > pj
		}
		return unique[i].ProfileID < unique[j].ProfileID
	})
	if len(unique) > limit {
		unique = unique[:limit]
	}
	return unique
}
