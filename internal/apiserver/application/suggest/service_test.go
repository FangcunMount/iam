package suggest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

type scopeStub struct{}

func (scopeStub) ResolveProfileAccessScope(context.Context, domainsuggest.OperatingPrincipal) (domainsuggest.ProfileAccessScope, error) {
	return domainsuggest.ProfileAccessScope{AllProfile: true, AllowMobileSearch: true}, nil
}

func TestServiceSuggestProfileReturnsEmptyWhenRuntimeHasNoCurrentIndex(t *testing.T) {
	service := NewServiceWithRuntime(Config{}, &suggestRuntimeStub{}, scopeStub{})

	got, err := service.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantID: 1, TenantDomain: "fangcun"},
		Keyword:   "张",
	})
	if err != nil {
		t.Fatalf("SuggestProfile() err = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SuggestProfile() = %#v, want empty", got)
	}
}

func TestProfileIndexRefresherFullSyncReplacesRuntimeIndex(t *testing.T) {
	runtime := &suggestRuntimeStub{}
	loader := &suggestLoaderStub{
		full: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 1, 0, nil),
		},
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
		delta: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 3, 1, 0, nil),
		},
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
		current: suggestIndexStub{terms: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 1, 0, nil),
		}},
	}
	loader := &suggestLoaderStub{
		delta: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 3, 1, 0, nil),
		},
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
		full: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 1, 0, nil),
		},
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
	full  []domainsuggest.ProfileSearchTerm
	delta []domainsuggest.ProfileSearchTerm
}

func (l *suggestLoaderStub) Full(context.Context) ([]domainsuggest.ProfileSearchTerm, error) {
	return append([]domainsuggest.ProfileSearchTerm(nil), l.full...), nil
}

func (l *suggestLoaderStub) Delta(context.Context, time.Time) ([]domainsuggest.ProfileSearchTerm, error) {
	return append([]domainsuggest.ProfileSearchTerm(nil), l.delta...), nil
}

type suggestRuntimeStub struct {
	current   ProfileSuggestionIndex
	replaced  []domainsuggest.ProfileSearchTerm
	imported  []domainsuggest.ProfileSearchTerm
	importErr error
}

func (r *suggestRuntimeStub) Current() ProfileSuggestionIndex {
	return r.current
}

func (r *suggestRuntimeStub) Replace(candidates []domainsuggest.ProfileSearchTerm) ProfileSuggestionIndex {
	r.replaced = append([]domainsuggest.ProfileSearchTerm(nil), candidates...)
	r.current = suggestIndexStub{terms: []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 1, 0, nil),
	}}
	return r.current
}

func (r *suggestRuntimeStub) ImportDelta(candidates []domainsuggest.ProfileSearchTerm) error {
	if r.importErr != nil {
		return r.importErr
	}
	r.imported = append([]domainsuggest.ProfileSearchTerm(nil), candidates...)
	return nil
}

type suggestIndexStub struct {
	terms []domainsuggest.ProfileSearchTerm
}

func (s suggestIndexStub) SuggestProfile(domainsuggest.Query, domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm {
	return append([]domainsuggest.ProfileSearchTerm(nil), s.terms...)
}

type snapshotWriterStub struct {
	written []domainsuggest.ProfileSearchTerm
}

func (s *snapshotWriterStub) Write(_ context.Context, candidates []domainsuggest.ProfileSearchTerm) error {
	s.written = append([]domainsuggest.ProfileSearchTerm(nil), candidates...)
	return nil
}
