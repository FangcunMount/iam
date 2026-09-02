package search

import (
	"sort"
	"strings"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

// SelectionOutcome 携带最终候选项与 scope 过滤统计。
type SelectionOutcome struct {
	Profiles     []profile.SuggestibleProfile
	MatchedCount int
	VisibleCount int
}

// SelectionPolicy 固定 scope filter → ProfileID 去重 → 排序 → final limit。
type SelectionPolicy struct{}

// Select 在 scope 内去重、排序并截断。
func (SelectionPolicy) Select(
	candidates []Candidate,
	scope visibility.Scope,
	keyword Keyword,
	limit int,
) SelectionOutcome {
	matched := len(candidates)
	if matched == 0 {
		return SelectionOutcome{Profiles: nil, MatchedCount: 0, VisibleCount: 0}
	}
	if limit <= 0 {
		limit = 20
	}

	visible := make([]Candidate, 0, min(len(candidates), limit))
	for _, c := range candidates {
		if scope.Allows(c.Profile.VisibilityResource()) {
			visible = append(visible, c)
		}
	}

	ranking := RankingPolicy{}
	profiles := ranking.RankForQuery(visible, keyword, limit)
	return SelectionOutcome{
		Profiles:     profiles,
		MatchedCount: matched,
		VisibleCount: len(visible),
	}
}

// RankingPolicy 在去重后按权重排序并截断。
type RankingPolicy struct{}

// RankForQuery 去重后按 Weight、MatchStrength、DisplayName 前缀、ProfileID 排序并截断。
func (RankingPolicy) RankForQuery(candidates []Candidate, keyword Keyword, limit int) []profile.SuggestibleProfile {
	if limit <= 0 {
		limit = 20
	}
	prefix := strings.TrimSpace(keyword.String())

	best := make(map[int64]Candidate, len(candidates))
	for _, c := range candidates {
		old, exists := best[c.Profile.ID()]
		if !exists || candidateBetter(c, old) {
			best[c.Profile.ID()] = c
		}
	}
	unique := make([]Candidate, 0, len(best))
	for _, c := range best {
		unique = append(unique, c)
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
		pi, pj := unique[i].Profile, unique[j].Profile
		if pi.Weight() != pj.Weight() {
			return pi.Weight() > pj.Weight()
		}
		si, sj := unique[i].Strength.RankPriority(), unique[j].Strength.RankPriority()
		if si != sj {
			return si > sj
		}
		bi, bj := prefixBonus(pi.DisplayName()), prefixBonus(pj.DisplayName())
		if bi != bj {
			return bi > bj
		}
		return pi.ID() < pj.ID()
	})

	out := make([]profile.SuggestibleProfile, 0, min(len(unique), limit))
	for i := 0; i < len(unique) && i < limit; i++ {
		out = append(out, unique[i].Profile)
	}
	return out
}

func candidateBetter(a, b Candidate) bool {
	if a.Profile.Weight() != b.Profile.Weight() {
		return a.Profile.Weight() > b.Profile.Weight()
	}
	return a.Strength.RankPriority() > b.Strength.RankPriority()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
