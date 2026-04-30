package suggest

import "testing"

func TestProfileCandidateNormalizesAndProjectsTerm(t *testing.T) {
	candidate := NewProfileCandidate(42, " 张三 ", []string{" 13800138000 ", "", "13900139000"}, 7)

	term := candidate.Term()

	if candidate.DisplayName != "张三" {
		t.Fatalf("DisplayName = %q, want 张三", candidate.DisplayName)
	}
	if len(candidate.Mobiles) != 2 || candidate.Mobiles[0] != "13800138000" || candidate.Mobiles[1] != "13900139000" {
		t.Fatalf("Mobiles = %#v, want normalized non-empty mobiles", candidate.Mobiles)
	}
	if term.Name != "张三" || term.ID != 42 || term.Mobile != "13800138000,13900139000" || term.Weight != 7 {
		t.Fatalf("Term() = %#v", term)
	}
}

func TestQueryDefaultsAndKeywordDigits(t *testing.T) {
	query := NewQuery(" 123 ", 0, 0)

	if query.Keyword.String() != "123" {
		t.Fatalf("Keyword = %q, want 123", query.Keyword.String())
	}
	if !query.Keyword.IsDigits() {
		t.Fatalf("Keyword.IsDigits() = false, want true")
	}
	if query.Limit != DefaultLimit || query.KeyPadLen != DefaultKeyPadLen {
		t.Fatalf("defaults = (%d,%d), want (%d,%d)", query.Limit, query.KeyPadLen, DefaultLimit, DefaultKeyPadLen)
	}
	if NewKeyword("12a").IsDigits() {
		t.Fatalf("non-numeric keyword reported as digits")
	}
	if NewKeyword("").IsDigits() {
		t.Fatalf("empty keyword reported as digits")
	}
}

func TestRankingPolicyDeduplicatesSortsAndLimits(t *testing.T) {
	terms := []Term{
		{ID: 1, Name: "first", Weight: 1},
		{ID: 2, Name: "second", Weight: 9},
		{ID: 1, Name: "duplicate", Weight: 99},
		{ID: 3, Name: "third", Weight: 5},
	}

	got := RankingPolicy{}.Rank(terms, 2)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("ranked ids = [%d,%d], want [2,3]", got[0].ID, got[1].ID)
	}
}
