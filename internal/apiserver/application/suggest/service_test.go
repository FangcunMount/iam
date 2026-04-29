package suggest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
)

func TestServiceSuggestReturnsNilWhenRuntimeHasNoCurrentIndex(t *testing.T) {
	service := NewServiceWithRuntime(Config{}, &suggestRuntimeStub{})

	if got := service.Suggest(context.Background(), "张"); got != nil {
		t.Fatalf("Suggest() = %#v, want nil", got)
	}
}

func TestUpdaterFullSyncReplacesRuntimeIndex(t *testing.T) {
	runtime := &suggestRuntimeStub{}
	loader := &suggestLoaderStub{
		full: []string{"张三|1|13800138000|-|5"},
	}
	updater := NewUpdaterWithRuntime(loader, UpdaterConfig{}, runtime)

	if err := updater.runFull(context.Background()); err != nil {
		t.Fatalf("runFull() error = %v", err)
	}

	if !reflect.DeepEqual(runtime.replaced, loader.full) {
		t.Fatalf("replaced = %#v, want %#v", runtime.replaced, loader.full)
	}
	if runtime.current == nil {
		t.Fatalf("current index was not installed")
	}
}

func TestUpdaterDeltaSyncReturnsRuntimeNotInitializedError(t *testing.T) {
	wantErr := errors.New("suggest store not initialized")
	runtime := &suggestRuntimeStub{importErr: wantErr}
	loader := &suggestLoaderStub{
		delta: []string{"李四|2|13900139000|-|3"},
	}
	updater := NewUpdaterWithRuntime(loader, UpdaterConfig{}, runtime)
	updater.lastFetch = time.Now().Add(-time.Minute)

	err := updater.runDelta(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("runDelta() error = %v, want %v", err, wantErr)
	}
}

func TestUpdaterDeltaSyncAppendsToCurrentIndex(t *testing.T) {
	runtime := &suggestRuntimeStub{
		current: suggestIndexStub{terms: []domainsuggest.Term{{Name: "张三", ID: 1, Weight: 5}}},
	}
	loader := &suggestLoaderStub{
		delta: []string{"李四|2|13900139000|-|3"},
	}
	updater := NewUpdaterWithRuntime(loader, UpdaterConfig{}, runtime)
	updater.lastFetch = time.Now().Add(-time.Minute)

	if err := updater.runDelta(context.Background()); err != nil {
		t.Fatalf("runDelta() error = %v", err)
	}

	if !reflect.DeepEqual(runtime.imported, loader.delta) {
		t.Fatalf("imported = %#v, want %#v", runtime.imported, loader.delta)
	}
}

type suggestLoaderStub struct {
	full  []string
	delta []string
}

func (l *suggestLoaderStub) Full(context.Context) ([]string, error) {
	return append([]string(nil), l.full...), nil
}

func (l *suggestLoaderStub) Delta(context.Context, time.Time) ([]string, error) {
	return append([]string(nil), l.delta...), nil
}

type suggestRuntimeStub struct {
	current   SearchIndex
	replaced  []string
	imported  []string
	importErr error
}

func (r *suggestRuntimeStub) Current() SearchIndex {
	return r.current
}

func (r *suggestRuntimeStub) Replace(lines []string) SearchIndex {
	r.replaced = append([]string(nil), lines...)
	r.current = suggestIndexStub{terms: []domainsuggest.Term{{Name: "张三", ID: 1, Weight: 5}}}
	return r.current
}

func (r *suggestRuntimeStub) ImportDelta(lines []string) error {
	if r.importErr != nil {
		return r.importErr
	}
	r.imported = append([]string(nil), lines...)
	return nil
}

type suggestIndexStub struct {
	terms []domainsuggest.Term
}

func (s suggestIndexStub) Suggest(string, int, int) []domainsuggest.Term {
	return append([]domainsuggest.Term(nil), s.terms...)
}
