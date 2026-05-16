package suggest

// MatchKind 表示索引召回来源，用于同权重时的排序优先级。
type MatchKind int

const (
	// MatchKindWildcard 通配展开键命中。
	MatchKindWildcard MatchKind = iota
	// MatchKindPrefix 前缀键直接命中（含补齐后的查询键）。
	MatchKindPrefix
	// MatchKindExact 精确匹配（档案 ID 或手机号）。
	MatchKindExact
)

// RankPriority 越大越优先。
func (k MatchKind) RankPriority() int {
	switch k {
	case MatchKindExact:
		return 3
	case MatchKindPrefix:
		return 2
	default:
		return 1
	}
}

// RankedProfileSearchTerm 带召回来源的候选项，供排序使用。
type RankedProfileSearchTerm struct {
	Term ProfileSearchTerm
	Kind MatchKind
}
