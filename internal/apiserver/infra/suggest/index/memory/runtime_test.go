package memory

import (
	"context"
	"testing"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

func TestRuntimeRecallUninitializedReturnsEmpty(t *testing.T) {
	r := NewRuntime(Config{})
	got, err := r.Recall(context.Background(), appquery.RecallRequest{
		Keyword: domainsearch.NewKeyword("张"),
		Intent:  domainsearch.IntentTextPrefix,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want empty", got)
	}
}

func TestRuntimeReplaceInstallsStore(t *testing.T) {
	r := NewRuntime(Config{})
	profiles := []profile.SuggestibleProfile{
		profile.New(1, "张三", nil, 5, 0, nil),
	}
	if err := r.Replace(context.Background(), profiles); err != nil {
		t.Fatal(err)
	}
	store := r.CurrentStore()
	if store == nil || store.Len() != 1 {
		t.Fatalf("store = %#v", store)
	}
	got, err := r.Recall(context.Background(), appquery.RecallRequest{
		Keyword: domainsearch.NewKeyword("张"),
		Intent:  domainsearch.IntentTextPrefix,
	})
	if err != nil || len(got) != 1 {
		t.Fatalf("Recall = %#v err=%v", got, err)
	}
}

func TestRuntimeApplyWithoutInitializationFails(t *testing.T) {
	r := NewRuntime(Config{})
	upsert, err := apprefresh.Upsert(profile.New(1, "a", nil, 1, 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	err = r.Apply(context.Background(), []apprefresh.ProjectionChange{upsert})
	if err == nil || err.Error() != "suggest store not initialized" {
		t.Fatalf("Apply() error = %v, want store not initialized", err)
	}
}

func TestRuntimeApplyAppendsToCurrentIndex(t *testing.T) {
	r := NewRuntime(Config{})
	if err := r.Replace(context.Background(), []profile.SuggestibleProfile{
		profile.New(1, "张三", nil, 5, 0, nil),
	}); err != nil {
		t.Fatal(err)
	}
	upsert, err := apprefresh.Upsert(profile.New(2, "李四", []string{"13900139000"}, 3, 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(context.Background(), []apprefresh.ProjectionChange{upsert}); err != nil {
		t.Fatal(err)
	}
	if r.CurrentStore().Len() != 2 {
		t.Fatalf("Len = %d, want 2", r.CurrentStore().Len())
	}
}

func TestRuntimeReplaceNilRuntimeFails(t *testing.T) {
	var r *Runtime
	err := r.Replace(context.Background(), nil)
	if err == nil || err.Error() != "suggest runtime is nil" {
		t.Fatalf("Replace() error = %v", err)
	}
}
