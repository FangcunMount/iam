package memory

import (
	"context"
	"testing"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

func recall(t *testing.T, store *Store, keyword string, intent domainsearch.Intent, budget int) []domainsearch.Candidate {
	t.Helper()
	out, err := store.Recall(context.Background(), appquery.RecallRequest{
		Keyword:         domainsearch.NewKeyword(keyword),
		Intent:          intent,
		CandidateBudget: budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRecallByPrefixAndPinyin(t *testing.T) {
	store := Load([]profile.SuggestibleProfile{
		profile.MustNew(1, "张三", []string{"13800138000"}, 5, 0, nil),
		profile.MustNew(3, "张三丰", []string{"18888888888"}, 8, 0, nil),
		profile.MustNew(2, "李四", []string{"13900139000"}, 3, 0, nil),
	}, Config{})

	out := recall(t, store, "张", domainsearch.IntentTextPrefix, 50)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	ids := map[int64]struct{}{}
	for _, c := range out {
		ids[c.Profile.ID()] = struct{}{}
	}
	if _, ok := ids[1]; !ok {
		t.Fatalf("missing profile 1: %+v", out)
	}
	if _, ok := ids[3]; !ok {
		t.Fatalf("missing profile 3: %+v", out)
	}

	abbr := recall(t, store, "zsf", domainsearch.IntentTextPrefix, 50)
	if len(abbr) != 1 || abbr[0].Profile.ID() != 3 {
		t.Fatalf("abbr expected id 3, got %+v", abbr)
	}

	pinyin := recall(t, store, "zhang", domainsearch.IntentTextPrefix, 50)
	if len(pinyin) != 2 {
		t.Fatalf("pinyin expected 2 results, got %d", len(pinyin))
	}
}

func TestRecallNumericExactDedupAndSort(t *testing.T) {
	store := Load([]profile.SuggestibleProfile{
		profile.MustNew(1, "张三", []string{"13900139000"}, 5, 0, nil),
		profile.MustNew(2, "李四", []string{"13900139000"}, 10, 0, nil),
		profile.MustNew(3, "王五", []string{"18800001111"}, 1, 0, nil),
	}, Config{})

	out := recall(t, store, "13900139000", domainsearch.IntentNumericExact, 50)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
}

func TestRecallNumericExactByID(t *testing.T) {
	store := Load([]profile.SuggestibleProfile{
		profile.MustNew(42, "张三", nil, 5, 0, nil),
	}, Config{})

	out := recall(t, store, "42", domainsearch.IntentNumericExact, 50)
	if len(out) != 1 || out[0].Profile.ID() != 42 {
		t.Fatalf("got %+v", out)
	}
}

func TestRecallCandidateBudget(t *testing.T) {
	profiles := make([]profile.SuggestibleProfile, 0, 10)
	for i := int64(1); i <= 10; i++ {
		profiles = append(profiles, profile.MustNew(i, "张"+string(rune('0'+i)), nil, int(i), 0, nil))
	}
	store := Load(profiles, Config{})

	out := recall(t, store, "张", domainsearch.IntentTextPrefix, 3)
	if len(out) != 3 {
		t.Fatalf("expected budget cap 3, got %d", len(out))
	}
}

func TestApplyChangesClearsStaleTrieKeys(t *testing.T) {
	s := Load([]profile.SuggestibleProfile{
		profile.MustNew(1, "张三", nil, 5, 0, nil),
	}, Config{})
	s.ApplyChanges([]apprefresh.ProjectionChange{
		mustUpsert(profile.MustNew(1, "李四", nil, 5, 0, nil)),
	})

	out := recall(t, s, "张", domainsearch.IntentTextPrefix, 50)
	if len(out) != 0 {
		t.Fatalf("stale trie keys still match 张: %#v", out)
	}
	out2 := recall(t, s, "李", domainsearch.IntentTextPrefix, 50)
	if len(out2) != 1 || out2[0].Profile.DisplayName() != "李四" {
		t.Fatalf("got %#v", out2)
	}
}

func TestApplyChangesEmptyDisplayRemovesProfile(t *testing.T) {
	s := Load([]profile.SuggestibleProfile{
		profile.MustNew(1, "张三", nil, 5, 0, nil),
	}, Config{})
	del, err := apprefresh.Delete(1)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyChanges([]apprefresh.ProjectionChange{del})

	out := recall(t, s, "张", domainsearch.IntentTextPrefix, 50)
	if len(out) != 0 {
		t.Fatalf("expected removal, got %#v", out)
	}
	if s.Len() != 0 {
		t.Fatalf("Len = %d", s.Len())
	}
}

func TestApplyChangesRepeatedTombstoneIsIdempotent(t *testing.T) {
	s := Load([]profile.SuggestibleProfile{
		profile.MustNew(1, "张三", nil, 5, 0, nil),
	}, Config{})
	del, err := apprefresh.Delete(1)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyChanges([]apprefresh.ProjectionChange{del})
	s.ApplyChanges([]apprefresh.ProjectionChange{del})
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0 after repeated tombstone", s.Len())
	}
}

func TestApplyChangesMobileChangeRevokesOldHashKey(t *testing.T) {
	s := Load([]profile.SuggestibleProfile{
		profile.MustNew(1, "张三", []string{"13800138000"}, 5, 0, nil),
	}, Config{})
	s.ApplyChanges([]apprefresh.ProjectionChange{
		mustUpsert(profile.MustNew(1, "张三", []string{"13900139000"}, 5, 0, nil)),
	})

	old := recall(t, s, "13800138000", domainsearch.IntentNumericExact, 50)
	if len(old) != 0 {
		t.Fatalf("old mobile still matches: %#v", old)
	}
	nu := recall(t, s, "13900139000", domainsearch.IntentNumericExact, 50)
	if len(nu) != 1 || nu[0].Profile.ID() != 1 {
		t.Fatalf("new mobile = %#v", nu)
	}
}

func TestReplaceProfilesReplacesIndex(t *testing.T) {
	s := Load([]profile.SuggestibleProfile{
		profile.MustNew(1, "张三", nil, 5, 0, nil),
	}, Config{})
	s.ReplaceProfiles([]profile.SuggestibleProfile{
		profile.MustNew(2, "李四", nil, 3, 0, nil),
	})
	if s.Len() != 1 {
		t.Fatalf("Len = %d", s.Len())
	}
	out := recall(t, s, "李", domainsearch.IntentTextPrefix, 50)
	if len(out) != 1 || out[0].Profile.ID() != 2 {
		t.Fatalf("got %+v", out)
	}
}

func mustUpsert(p profile.SuggestibleProfile) apprefresh.ProjectionChange {
	c, err := apprefresh.Upsert(p)
	if err != nil {
		panic(err)
	}
	return c
}
