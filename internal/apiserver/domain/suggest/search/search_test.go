package search_test

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

func TestAdmissionPolicyMatrix(t *testing.T) {
	policy := search.AdmissionPolicy{}
	allScope := visibility.NewScope(true, true, 0, nil, nil)
	noMobile := visibility.NewScope(true, false, 0, nil, nil)

	cases := []struct {
		keyword string
		scope   visibility.Scope
		allowed bool
		intent  search.Intent
	}{
		{"", allScope, false, search.IntentNone},
		{"13800138000", noMobile, false, search.IntentNone},
		{"13800138000", allScope, true, search.IntentNumericExact},
		{"42", allScope, true, search.IntentNumericExact},
		{"张", allScope, true, search.IntentTextPrefix},
	}
	for _, tc := range cases {
		d := policy.Decide(search.NewKeyword(tc.keyword), tc.scope)
		if d.Allowed() != tc.allowed {
			t.Fatalf("keyword=%q allowed=%v want %v", tc.keyword, d.Allowed(), tc.allowed)
		}
		if d.Intent() != tc.intent {
			t.Fatalf("keyword=%q intent=%v want %v", tc.keyword, d.Intent(), tc.intent)
		}
	}
}

func TestSelectionRankingMatchStrengthPriority(t *testing.T) {
	candidates := []search.Candidate{
		{Profile: profile.MustNew(1, "wildcard-hit", nil, 5, 0, nil), Strength: search.MatchExpandedPrefix},
		{Profile: profile.MustNew(2, "exact-hit", nil, 5, 0, nil), Strength: search.MatchExact},
	}
	out := search.SelectionPolicy{}.Select(candidates, visibility.NewScope(true, true, 0, nil, nil), search.NewKeyword("张"), 10)
	if len(out.Profiles) != 2 || out.Profiles[0].ID() != 2 {
		t.Fatalf("got %+v, want exact match first", out.Profiles)
	}
}

func TestSelectionKeepsBestWeightAndLimits(t *testing.T) {
	candidates := []search.Candidate{
		{Profile: profile.MustNew(1, "first", nil, 1, 0, nil), Strength: search.MatchDirectPrefix},
		{Profile: profile.MustNew(2, "second", nil, 9, 0, nil), Strength: search.MatchDirectPrefix},
		{Profile: profile.MustNew(1, "duplicate", nil, 99, 0, nil), Strength: search.MatchDirectPrefix},
	}
	out := search.SelectionPolicy{}.Select(candidates, visibility.NewScope(true, true, 0, nil, nil), search.NewKeyword(""), 2)
	if len(out.Profiles) != 2 || out.Profiles[0].ID() != 1 || out.Profiles[1].ID() != 2 {
		t.Fatalf("got %+v", out.Profiles)
	}
	if out.Profiles[0].Weight() != 99 {
		t.Fatalf("weight = %d", out.Profiles[0].Weight())
	}
}
