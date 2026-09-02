package suggest

import "testing"

func TestCandidateSelectionPolicyFiltersBeforeLimit(t *testing.T) {
	terms := []RankedProfileSearchTerm{
		{Term: ProfileSearchTerm{ProfileID: 1, DisplayName: "张三", Weight: 5, OrgID: 1}, Kind: MatchKindPrefix},
		{Term: ProfileSearchTerm{ProfileID: 2, DisplayName: "李四", Weight: 10, OrgID: 2}, Kind: MatchKindPrefix},
		{Term: ProfileSearchTerm{ProfileID: 3, DisplayName: "王五", Weight: 8, OrgID: 1}, Kind: MatchKindPrefix},
	}
	q := NewQuery("张", 1, 50, 8, 0)
	q.SearchMode = SearchModePrefix

	result := NewCandidateSelectionPolicy().Select(terms, ProfileAccessScope{OrgIDs: []int64{1}}, q)
	if result.MatchedCount != 3 || result.VisibleCount != 2 || len(result.Terms) != 1 {
		t.Fatalf("result = matched:%d visible:%d terms:%d", result.MatchedCount, result.VisibleCount, len(result.Terms))
	}
	if result.Terms[0].ProfileID != 3 {
		t.Fatalf("first term = %d, want 3 (highest weight in org 1)", result.Terms[0].ProfileID)
	}
}
