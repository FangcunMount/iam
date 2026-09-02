package search

import "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"

// MatchStrength 表示召回匹配强度，用于排序。
type MatchStrength uint8

const (
	MatchExpandedPrefix MatchStrength = iota + 1
	MatchDirectPrefix
	MatchExact
)

// RankPriority 越大越优先。
func (s MatchStrength) RankPriority() int {
	switch s {
	case MatchExact:
		return 3
	case MatchDirectPrefix:
		return 2
	default:
		return 1
	}
}

// Candidate 是带匹配强度的候选项。
type Candidate struct {
	Profile  profile.SuggestibleProfile
	Strength MatchStrength
}
