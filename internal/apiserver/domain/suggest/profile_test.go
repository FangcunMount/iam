package suggest

import "testing"

func TestProfileSearchTermNormalizes(t *testing.T) {
	term := NewProfileSearchTerm(42, " 张三 ", []string{" 13800138000 ", "", "13900139000"}, 7, 0, []int64{99, 99, 0})

	if term.DisplayName != "张三" {
		t.Fatalf("DisplayName = %q, want 张三", term.DisplayName)
	}
	if len(term.Mobiles) != 2 || term.Mobiles[0] != "13800138000" || term.Mobiles[1] != "13900139000" {
		t.Fatalf("Mobiles = %#v, want normalized non-empty mobiles", term.Mobiles)
	}
	if term.ProfileID != 42 || term.Weight != 7 || term.OrgID != 0 {
		t.Fatalf("fields = (%d,%d,%d)", term.ProfileID, term.Weight, term.OrgID)
	}
	if len(term.OwnerOperatorIDs) != 1 || term.OwnerOperatorIDs[0] != 99 {
		t.Fatalf("OwnerOperatorIDs = %#v", term.OwnerOperatorIDs)
	}
}

func TestQueryDefaultsAndKeywordDigits(t *testing.T) {
	query := NewQuery(" 123 ", 0, 0, 0, 0)

	if query.Keyword.String() != "123" {
		t.Fatalf("Keyword = %q, want 123", query.Keyword.String())
	}
	if !query.Keyword.IsDigits() {
		t.Fatalf("Keyword.IsDigits() = false, want true")
	}
	if query.Limit != DefaultLimit || query.KeyPadLen != DefaultKeyPadLen {
		t.Fatalf("defaults = (%d,%d), want (%d,%d)", query.Limit, query.KeyPadLen, DefaultLimit, DefaultKeyPadLen)
	}
	if query.WildcardKeyCap != DefaultWildcardKeyCap {
		t.Fatalf("WildcardKeyCap = %d, want %d", query.WildcardKeyCap, DefaultWildcardKeyCap)
	}
	if query.InternalLimit < query.Limit {
		t.Fatalf("InternalLimit = %d < limit %d", query.InternalLimit, query.Limit)
	}
	if NewKeyword("12a").IsDigits() {
		t.Fatalf("non-numeric keyword reported as digits")
	}
	if NewKeyword("").IsDigits() {
		t.Fatalf("empty keyword reported as digits")
	}
}

func TestRankingPolicyKeepsBestWeightSortsAndLimits(t *testing.T) {
	terms := []ProfileSearchTerm{
		{ProfileID: 1, DisplayName: "first", Weight: 1},
		{ProfileID: 2, DisplayName: "second", Weight: 9},
		{ProfileID: 1, DisplayName: "duplicate", Weight: 99},
		{ProfileID: 3, DisplayName: "third", Weight: 5},
	}

	got := RankingPolicy{}.Rank(terms, 2)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ProfileID != 1 || got[1].ProfileID != 2 {
		t.Fatalf("ranked ids = [%d,%d], want [1,2]", got[0].ProfileID, got[1].ProfileID)
	}
	if got[0].Weight != 99 {
		t.Fatalf("id1 weight = %d, want 99", got[0].Weight)
	}
}

func TestRankingMatchKindPriority(t *testing.T) {
	terms := []RankedProfileSearchTerm{
		{Term: ProfileSearchTerm{ProfileID: 1, DisplayName: "wildcard-hit", Weight: 5}, Kind: MatchKindWildcard},
		{Term: ProfileSearchTerm{ProfileID: 2, DisplayName: "exact-hit", Weight: 5}, Kind: MatchKindExact},
	}
	q := NewQuery("张", 10, 50, 8, 0)
	got := RankingPolicy{}.RankRankedForQuery(terms, q)
	if len(got) != 2 || got[0].ProfileID != 2 {
		t.Fatalf("got %+v, want exact match first", got)
	}
}

func TestRankingPrefixBoost(t *testing.T) {
	terms := []ProfileSearchTerm{
		{ProfileID: 1, DisplayName: "三张", Weight: 5},
		{ProfileID: 2, DisplayName: "张三丰", Weight: 5},
	}
	q := NewQuery("张", 10, 50, 8, 0)
	got := RankingPolicy{}.RankForQuery(terms, q)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].ProfileID != 2 {
		t.Fatalf("got order %+v, want 张三丰 first", got)
	}
}

func TestQueryCustomWildcardCap(t *testing.T) {
	q := NewQuery("a", 5, 50, 8, 77)
	if q.WildcardKeyCap != 77 {
		t.Fatalf("WildcardKeyCap = %d", q.WildcardKeyCap)
	}
}
