package search

import (
	"testing"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

func TestSuggestByPrefixAndPinyin(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 1, 0, nil),
		domainsuggest.NewProfileSearchTerm(3, "张三丰", []string{"18888888888"}, 8, 1, 0, nil),
		domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 3, 1, 0, nil),
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13900000000"}, 5, 1, 0, nil), // duplicate ID should collapse by weight
	}

	store := Load(terms)
	all := domainsuggest.ProfileAccessScope{AllProfile: true}

	out := store.SuggestProfile(domainsuggest.NewQuery("张", 5, 50, 6), all)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].ProfileID != 3 {
		t.Fatalf("expected first id 3, got %d", out[0].ProfileID)
	}

	abbr := store.SuggestProfile(domainsuggest.NewQuery("zsf", 3, 50, 6), all)
	if len(abbr) != 1 || abbr[0].ProfileID != 3 {
		t.Fatalf("abbr expected id 3, got %+v", abbr)
	}

	pinyin := store.SuggestProfile(domainsuggest.NewQuery("zhang", 5, 50, 8), all)
	if len(pinyin) != 2 {
		t.Fatalf("pinyin expected 2 results, got %d", len(pinyin))
	}
}

func TestSuggestNumericDedupAndSort(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13900139000"}, 5, 1, 0, nil),
		domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 10, 1, 0, nil),
		domainsuggest.NewProfileSearchTerm(3, "王五", []string{"18800001111"}, 1, 1, 0, nil),
	}

	store := Load(terms)
	all := domainsuggest.ProfileAccessScope{AllProfile: true}

	out := store.SuggestProfile(domainsuggest.NewQuery("13900139000", 5, 50, 4), all)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].ProfileID != 2 || out[0].Weight != 10 {
		t.Fatalf("expected highest weight record first, got %+v", out[0])
	}
}

func TestSuggestProfileScopeFiltersByTenant(t *testing.T) {
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 1, 0, nil),
		domainsuggest.NewProfileSearchTerm(2, "李四", nil, 3, 2, 0, nil),
	}
	store := Load(terms)
	scope := domainsuggest.ProfileAccessScope{TenantIDs: []int64{1}}
	out := store.SuggestProfile(domainsuggest.NewQuery("张", 5, 50, 8), scope)
	if len(out) != 1 || out[0].ProfileID != 1 {
		t.Fatalf("got %+v", out)
	}
}
