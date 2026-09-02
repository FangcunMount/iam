package suggest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

type scopeStub struct{}

func (scopeStub) ResolveProfileAccessScope(context.Context, domainsuggest.OperatingPrincipal) (domainsuggest.ProfileAccessScope, error) {
	return domainsuggest.ProfileAccessScope{AllProfile: true, AllowMobileSearch: true}, nil
}

func TestServiceSuggestProfileReturnsEmptyWhenRuntimeHasNoCurrentIndex(t *testing.T) {
	service := NewServiceWithRuntime(Config{}, &suggestRuntimeStub{}, scopeStub{}, nil)

	got, err := service.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
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
			domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 0, nil),
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
	runtime := &suggestRuntimeStub{applyErr: wantErr}
	loader := &suggestLoaderStub{
		delta: []domainsuggest.ProfileIndexMutation{
			mustUpsert(domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 3, 0, nil)),
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
			domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 0, nil),
		}},
	}
	delta := []domainsuggest.ProfileIndexMutation{
		mustUpsert(domainsuggest.NewProfileSearchTerm(2, "李四", []string{"13900139000"}, 3, 0, nil)),
	}
	loader := &suggestLoaderStub{delta: delta}
	refresher := NewProfileIndexRefresher(loader, runtime, nil)
	refresher.lastFetch = time.Now().Add(-time.Minute)

	if err := refresher.RunDelta(context.Background()); err != nil {
		t.Fatalf("RunDelta() error = %v", err)
	}

	if !reflect.DeepEqual(runtime.applied, delta) {
		t.Fatalf("applied = %#v, want %#v", runtime.applied, delta)
	}
}

func mustUpsert(term domainsuggest.ProfileSearchTerm) domainsuggest.ProfileIndexMutation {
	m, err := domainsuggest.NewProfileIndexUpsert(term)
	if err != nil {
		panic(err)
	}
	return m
}

type suggestLoaderStub struct {
	full  []domainsuggest.ProfileSearchTerm
	delta []domainsuggest.ProfileIndexMutation
}

func (l *suggestLoaderStub) Full(context.Context) ([]domainsuggest.ProfileSearchTerm, error) {
	return append([]domainsuggest.ProfileSearchTerm(nil), l.full...), nil
}

func (l *suggestLoaderStub) Delta(context.Context, time.Time) ([]domainsuggest.ProfileIndexMutation, error) {
	return append([]domainsuggest.ProfileIndexMutation(nil), l.delta...), nil
}

type suggestRuntimeStub struct {
	current  ProfileSuggestionIndex
	replaced []domainsuggest.ProfileSearchTerm
	applied  []domainsuggest.ProfileIndexMutation
	applyErr error
}

func (r *suggestRuntimeStub) Current() ProfileSuggestionIndex {
	return r.current
}

func (r *suggestRuntimeStub) Replace(candidates []domainsuggest.ProfileSearchTerm) ProfileSuggestionIndex {
	r.replaced = append([]domainsuggest.ProfileSearchTerm(nil), candidates...)
	r.current = suggestIndexStub{terms: []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 0, nil),
	}}
	return r.current
}

func (r *suggestRuntimeStub) ApplyDelta(mutations []domainsuggest.ProfileIndexMutation) error {
	if r.applyErr != nil {
		return r.applyErr
	}
	r.applied = append([]domainsuggest.ProfileIndexMutation(nil), mutations...)
	return nil
}

type suggestIndexStub struct {
	terms []domainsuggest.ProfileSearchTerm
}

func (s suggestIndexStub) SuggestProfile(domainsuggest.Query, domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm {
	return append([]domainsuggest.ProfileSearchTerm(nil), s.terms...)
}
