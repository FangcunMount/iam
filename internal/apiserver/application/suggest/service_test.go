package suggest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

func TestServiceSuggestReturnsNilWhenRuntimeHasNoCurrentIndex(t *testing.T) {
	service := NewServiceWithRuntime(Config{}, &suggestRuntimeStub{})

	if got := service.Suggest(context.Background(), "张"); got != nil {
		t.Fatalf("Suggest() = %#v, want nil", got)
	}
}

func TestProfileIndexRefresherFullSyncReplacesRuntimeIndex(t *testing.T) {
	runtime := &suggestRuntimeStub{}
	loader := &suggestLoaderStub{
		full: []domainsuggest.ProfileCandidate{domainsuggest.NewProfileCandidate(1, "张三", []string{"13800138000"}, 5)},
	}
	refresher := NewProfileIndexRefresher(loader, runtime, nil)

	if err := refresher.RunFull(context.Background()); err != nil {
		t.Fatalf("RunFull() error = %v", err)
	}

	if !reflect.DeepEqual(runtime.replaced, loader.full) {
		t.Fatalf("replaced = %#v, want %#v", runtime.replaced, loader.full)
	}
	if runtime.current == nil {
		t.Fatalf("current index was not installed")
	}
}

func TestProfileIndexRefresherDeltaSyncReturnsRuntimeNotInitializedError(t *testing.T) {
	wantErr := errors.New("suggest store not initialized")
	runtime := &suggestRuntimeStub{importErr: wantErr}
	loader := &suggestLoaderStub{
		delta: []domainsuggest.ProfileCandidate{domainsuggest.NewProfileCandidate(2, "李四", []string{"13900139000"}, 3)},
	}
	refresher := NewProfileIndexRefresher(loader, runtime, nil)
	refresher.lastFetch = time.Now().Add(-time.Minute)

	err := refresher.RunDelta(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunDelta() error = %v, want %v", err, wantErr)
	}
}

func TestProfileIndexRefresherDeltaSyncAppendsToCurrentIndex(t *testing.T) {
	runtime := &suggestRuntimeStub{
		current: suggestIndexStub{terms: []domainsuggest.Term{{Name: "张三", ID: 1, Weight: 5}}},
	}
	loader := &suggestLoaderStub{
		delta: []domainsuggest.ProfileCandidate{domainsuggest.NewProfileCandidate(2, "李四", []string{"13900139000"}, 3)},
	}
	refresher := NewProfileIndexRefresher(loader, runtime, nil)
	refresher.lastFetch = time.Now().Add(-time.Minute)

	if err := refresher.RunDelta(context.Background()); err != nil {
		t.Fatalf("RunDelta() error = %v", err)
	}

	if !reflect.DeepEqual(runtime.imported, loader.delta) {
		t.Fatalf("imported = %#v, want %#v", runtime.imported, loader.delta)
	}
}

func TestRefresherWritesSnapshotAfterFullSync(t *testing.T) {
	runtime := &suggestRuntimeStub{}
	loader := &suggestLoaderStub{
		full: []domainsuggest.ProfileCandidate{domainsuggest.NewProfileCandidate(1, "张三", []string{"13800138000"}, 5)},
	}
	snapshot := &snapshotWriterStub{}
	refresher := NewProfileIndexRefresher(loader, runtime, snapshot)

	if err := refresher.RunFull(context.Background()); err != nil {
		t.Fatalf("RunFull() error = %v", err)
	}

	if !reflect.DeepEqual(snapshot.written, loader.full) {
		t.Fatalf("snapshot = %#v, want %#v", snapshot.written, loader.full)
	}
}

type suggestLoaderStub struct {
	full  []domainsuggest.ProfileCandidate
	delta []domainsuggest.ProfileCandidate
}

func (l *suggestLoaderStub) Full(context.Context) ([]domainsuggest.ProfileCandidate, error) {
	return append([]domainsuggest.ProfileCandidate(nil), l.full...), nil
}

func (l *suggestLoaderStub) Delta(context.Context, time.Time) ([]domainsuggest.ProfileCandidate, error) {
	return append([]domainsuggest.ProfileCandidate(nil), l.delta...), nil
}

type suggestRuntimeStub struct {
	current   ProfileSuggestionIndex
	replaced  []domainsuggest.ProfileCandidate
	imported  []domainsuggest.ProfileCandidate
	importErr error
}

func (r *suggestRuntimeStub) Current() ProfileSuggestionIndex {
	return r.current
}

func (r *suggestRuntimeStub) Replace(candidates []domainsuggest.ProfileCandidate) ProfileSuggestionIndex {
	r.replaced = append([]domainsuggest.ProfileCandidate(nil), candidates...)
	r.current = suggestIndexStub{terms: []domainsuggest.Term{{Name: "张三", ID: 1, Weight: 5}}}
	return r.current
}

func (r *suggestRuntimeStub) ImportDelta(candidates []domainsuggest.ProfileCandidate) error {
	if r.importErr != nil {
		return r.importErr
	}
	r.imported = append([]domainsuggest.ProfileCandidate(nil), candidates...)
	return nil
}

type suggestIndexStub struct {
	terms []domainsuggest.Term
}

func (s suggestIndexStub) Suggest(domainsuggest.Query) []domainsuggest.Term {
	return append([]domainsuggest.Term(nil), s.terms...)
}

type snapshotWriterStub struct {
	written []domainsuggest.ProfileCandidate
}

func (s *snapshotWriterStub) Write(_ context.Context, candidates []domainsuggest.ProfileCandidate) error {
	s.written = append([]domainsuggest.ProfileCandidate(nil), candidates...)
	return nil
}
