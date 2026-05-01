package search

import (
	"testing"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

func TestSuggestByPrefixAndPinyin(t *testing.T) {
	candidates := []domainsuggest.ProfileCandidate{
		domainsuggest.NewProfileCandidate(1, "张三", []string{"13800138000"}, 5),
		domainsuggest.NewProfileCandidate(3, "张三丰", []string{"18888888888"}, 8),
		domainsuggest.NewProfileCandidate(2, "李四", []string{"13900139000"}, 3),
		domainsuggest.NewProfileCandidate(1, "张三", []string{"13900000000"}, 5), // duplicate ID should be removed
	}

	store := Load(candidates)

	out := store.Suggest(domainsuggest.NewQuery("张", 5, 6))
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].ID != 3 {
		t.Fatalf("expected first id 3, got %d", out[0].ID)
	}

	abbr := store.Suggest(domainsuggest.NewQuery("zsf", 3, 6))
	if len(abbr) != 1 || abbr[0].ID != 3 {
		t.Fatalf("abbr expected id 3, got %+v", abbr)
	}

	pinyin := store.Suggest(domainsuggest.NewQuery("zhang", 5, 8))
	if len(pinyin) != 2 {
		t.Fatalf("pinyin expected 2 results, got %d", len(pinyin))
	}
}

func TestSuggestNumericDedupAndSort(t *testing.T) {
	candidates := []domainsuggest.ProfileCandidate{
		domainsuggest.NewProfileCandidate(1, "张三", []string{"13900139000"}, 5),
		domainsuggest.NewProfileCandidate(2, "李四", []string{"13900139000"}, 10),
		domainsuggest.NewProfileCandidate(3, "王五", []string{"18800001111"}, 1),
	}

	store := Load(candidates)

	out := store.Suggest(domainsuggest.NewQuery("13900139000", 5, 4))
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].ID != 2 || out[0].Weight != 10 {
		t.Fatalf("expected highest weight record first, got %+v", out[0])
	}
}
