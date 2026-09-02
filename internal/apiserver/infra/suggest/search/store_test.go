package search

import (
	"testing"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

func TestSuggestByPrefixAndPinyin(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 0, nil),
		domainsuggest.NewProfileSearchTerm(3, "张三丰", []string{"18888888888"}, 8, 0, nil),
		domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 3, 0, nil),
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13900000000"}, 5, 0, nil),
	}

	store := Load(terms)
	all := domainsuggest.ProfileAccessScope{AllProfile: true}

	out := store.SuggestProfile(domainsuggest.NewQuery("张", 5, 50, 6, 0), all)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].ProfileID != 3 {
		t.Fatalf("expected first id 3, got %d", out[0].ProfileID)
	}

	abbr := store.SuggestProfile(domainsuggest.NewQuery("zsf", 3, 50, 6, 0), all)
	if len(abbr) != 1 || abbr[0].ProfileID != 3 {
		t.Fatalf("abbr expected id 3, got %+v", abbr)
	}

	pinyin := store.SuggestProfile(domainsuggest.NewQuery("zhang", 5, 50, 8, 0), all)
	if len(pinyin) != 2 {
		t.Fatalf("pinyin expected 2 results, got %d", len(pinyin))
	}
}

func TestSuggestNumericDedupAndSort(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13900139000"}, 5, 0, nil),
		domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 10, 0, nil),
		domainsuggest.NewProfileSearchTerm(3, "王五", []string{"18800001111"}, 1, 0, nil),
	}

	store := Load(terms)
	all := domainsuggest.ProfileAccessScope{AllProfile: true}

	out := store.SuggestProfile(domainsuggest.NewQuery("13900139000", 5, 50, 4, 0), all)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].ProfileID != 2 || out[0].Weight != 10 {
		t.Fatalf("expected highest weight record first, got %+v", out[0])
	}
}

func TestSuggestProfileScopeFiltersByOrg(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 1, nil),
		domainsuggest.NewProfileSearchTerm(2, "李四", nil, 3, 2, nil),
	}
	store := Load(terms)
	scope := domainsuggest.ProfileAccessScope{OrgIDs: []int64{1}}
	out := store.SuggestProfile(domainsuggest.NewQuery("张", 5, 50, 8, 0), scope)
	if len(out) != 1 || out[0].ProfileID != 1 {
		t.Fatalf("got %+v", out)
	}
}

func TestSuggestProfileScopeFiltersByOrgOperatorProfileIDs(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张伟", nil, 5, 10, []int64{200}),
		domainsuggest.NewProfileSearchTerm(2, "张磊", nil, 5, 20, []int64{100}),
	}
	store := Load(terms)
	q := domainsuggest.NewQuery("张", 5, 50, 8, 0)

	outOrg := store.SuggestProfile(q, domainsuggest.ProfileAccessScope{OrgIDs: []int64{10}})
	if len(outOrg) != 1 || outOrg[0].ProfileID != 1 {
		t.Fatalf("org scope got %+v", outOrg)
	}

	outOp := store.SuggestProfile(q, domainsuggest.ProfileAccessScope{OperatorID: 100})
	if len(outOp) != 1 || outOp[0].ProfileID != 2 {
		t.Fatalf("operator scope got %+v", outOp)
	}

	outPID := store.SuggestProfile(q, domainsuggest.ProfileAccessScope{ProfileIDs: []int64{2}})
	if len(outPID) != 1 || outPID[0].ProfileID != 2 {
		t.Fatalf("profileIDs scope got %+v", outPID)
	}

	outEmpty := store.SuggestProfile(q, domainsuggest.ProfileAccessScope{})
	if len(outEmpty) != 0 {
		t.Fatalf("empty scope expected no hits, got %+v", outEmpty)
	}
}

func TestSuggestProfileScopeFiltersByOwnerOperator(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张伟", nil, 5, 0, []int64{100}),
		domainsuggest.NewProfileSearchTerm(2, "张磊", nil, 5, 0, []int64{200}),
	}
	store := Load(terms)
	q := domainsuggest.NewQuery("张", 5, 50, 8, 0)

	outOp := store.SuggestProfile(q, domainsuggest.ProfileAccessScope{OperatorID: 100})
	if len(outOp) != 1 || outOp[0].ProfileID != 1 {
		t.Fatalf("operator scope got %+v", outOp)
	}

	outPID := store.SuggestProfile(q, domainsuggest.ProfileAccessScope{ProfileIDs: []int64{2}})
	if len(outPID) != 1 || outPID[0].ProfileID != 2 {
		t.Fatalf("profileIDs scope got %+v", outPID)
	}
}

func TestImportTermsClearsStaleTrieKeys(t *testing.T) {
	s := Load([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 0, nil),
	})
	s.ImportTerms([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "李四", nil, 5, 0, nil),
	})
	all := domainsuggest.ProfileAccessScope{AllProfile: true}
	out := s.SuggestProfile(domainsuggest.NewQuery("张", 5, 50, 8, 0), all)
	if len(out) != 0 {
		t.Fatalf("stale trie keys still match 张: %#v", out)
	}
	out2 := s.SuggestProfile(domainsuggest.NewQuery("李", 5, 50, 8, 0), all)
	if len(out2) != 1 || out2[0].DisplayName != "李四" {
		t.Fatalf("got %#v", out2)
	}
}

func TestImportTermsEmptyDisplayRemovesProfile(t *testing.T) {
	s := Load([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 0, nil),
	})
	s.ImportTerms([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "", nil, 5, 0, nil),
	})
	all := domainsuggest.ProfileAccessScope{AllProfile: true}
	out := s.SuggestProfile(domainsuggest.NewQuery("张", 5, 50, 8, 0), all)
	if len(out) != 0 {
		t.Fatalf("expected removal, got %#v", out)
	}
	if s.Len() != 0 {
		t.Fatalf("Len = %d", s.Len())
	}
}

func TestImportTermsRepeatedTombstoneIsIdempotent(t *testing.T) {
	s := Load([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 0, nil),
	})
	tomb := domainsuggest.NewProfileSearchTerm(1, "", nil, 5, 0, nil)
	s.ImportTerms([]domainsuggest.ProfileSearchTerm{tomb})
	s.ImportTerms([]domainsuggest.ProfileSearchTerm{tomb})
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0 after repeated tombstone", s.Len())
	}
}

func TestImportTermsMobileChangeRevokesOldHashKey(t *testing.T) {
	s := Load([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 0, nil),
	})
	s.ImportTerms([]domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13900139000"}, 5, 0, nil),
	})
	all := domainsuggest.ProfileAccessScope{AllProfile: true}
	old := s.SuggestProfile(domainsuggest.NewQuery("13800138000", 5, 50, 8, 0), all)
	if len(old) != 0 {
		t.Fatalf("old mobile still matches: %#v", old)
	}
	nu := s.SuggestProfile(domainsuggest.NewQuery("13900139000", 5, 50, 8, 0), all)
	if len(nu) != 1 || nu[0].ProfileID != 1 {
		t.Fatalf("new mobile = %#v", nu)
	}
}
