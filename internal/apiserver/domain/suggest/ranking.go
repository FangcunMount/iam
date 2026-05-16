package suggest

import "sort"

// RankingPolicy 在去重后按权重排序并截断；同一 ProfileID 保留权重更高的项。
type RankingPolicy struct{}

// Rank 对 ProfileSearchTerm 去重、排序并 limit。
func (RankingPolicy) Rank(terms []ProfileSearchTerm, limit int) []ProfileSearchTerm {
	if limit <= 0 {
		limit = DefaultLimit
	}
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
	sort.SliceStable(unique, func(i, j int) bool {
		if unique[i].Weight != unique[j].Weight {
			return unique[i].Weight > unique[j].Weight
		}
		return unique[i].ProfileID < unique[j].ProfileID
	})
	if len(unique) > limit {
		unique = unique[:limit]
	}
	return unique
}
