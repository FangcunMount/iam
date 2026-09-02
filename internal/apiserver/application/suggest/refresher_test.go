package suggest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

func TestProfileIndexRefresherFullUsesQueryStartAsDeltaCursor(t *testing.T) {
	queryStart := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	refresher := NewProfileIndexRefresher(&suggestLoaderStub{}, &suggestRuntimeStub{}, nil)
	refresher.now = func() time.Time { return queryStart }

	if err := refresher.RunFull(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !refresher.lastFetch.Equal(queryStart) {
		t.Fatalf("lastFetch = %s, want query start %s", refresher.lastFetch, queryStart)
	}
}

func TestProfileIndexRefresherEmptyDeltaAdvancesCursor(t *testing.T) {
	previous := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	windowStart := previous.Add(time.Minute)
	loader := &recordingSuggestLoader{}
	refresher := NewProfileIndexRefresher(loader, &suggestRuntimeStub{}, nil)
	refresher.lastFetch = previous
	refresher.now = func() time.Time { return windowStart }

	if err := refresher.RunDelta(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !loader.since.Equal(previous) {
		t.Fatalf("Delta since = %s, want %s", loader.since, previous)
	}
	if !refresher.lastFetch.Equal(windowStart) {
		t.Fatalf("lastFetch = %s, want %s", refresher.lastFetch, windowStart)
	}
}

func TestProfileIndexRefresherFailureDoesNotAdvanceCursor(t *testing.T) {
	previous := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	wantErr := errors.New("import failed")
	loader := &recordingSuggestLoader{
		delta: []domainsuggest.ProfileIndexMutation{
			mustUpsert(domainsuggest.NewProfileSearchTerm(1, "profile", nil, 1, 0, nil)),
		},
	}
	refresher := NewProfileIndexRefresher(loader, &suggestRuntimeStub{applyErr: wantErr}, nil)
	refresher.lastFetch = previous
	refresher.now = func() time.Time { return previous.Add(time.Minute) }

	if err := refresher.RunDelta(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("RunDelta() error = %v, want %v", err, wantErr)
	}
	if !refresher.lastFetch.Equal(previous) {
		t.Fatalf("lastFetch advanced to %s after failure; want %s", refresher.lastFetch, previous)
	}
}

func TestProfileIndexRefresherRejectsOverlappingRefreshes(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	loader := &blockingSuggestLoader{entered: entered, release: release}
	refresher := NewProfileIndexRefresher(loader, &suggestRuntimeStub{}, nil)
	refresher.lastFetch = time.Now().Add(-time.Minute)

	var wg sync.WaitGroup
	wg.Add(1)
	var firstErr error
	go func() {
		defer wg.Done()
		firstErr = refresher.RunFull(context.Background())
	}()
	<-entered

	if err := refresher.RunDelta(context.Background()); !errors.Is(err, ErrRefreshInProgress) {
		t.Fatalf("overlapping RunDelta() error = %v, want %v", err, ErrRefreshInProgress)
	}
	close(release)
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("RunFull() error = %v", firstErr)
	}
	if loader.fullCalls != 1 || loader.deltaCalls != 0 {
		t.Fatalf("loader calls full=%d delta=%d, want full=1 delta=0", loader.fullCalls, loader.deltaCalls)
	}
}

type recordingSuggestLoader struct {
	since time.Time
	delta []domainsuggest.ProfileIndexMutation
}

func (l *recordingSuggestLoader) Full(context.Context) ([]domainsuggest.ProfileSearchTerm, error) {
	return nil, nil
}

func (l *recordingSuggestLoader) Delta(_ context.Context, since time.Time) ([]domainsuggest.ProfileIndexMutation, error) {
	l.since = since
	return append([]domainsuggest.ProfileIndexMutation(nil), l.delta...), nil
}

type blockingSuggestLoader struct {
	entered chan struct{}
	release chan struct{}

	fullCalls  int
	deltaCalls int
}

func (l *blockingSuggestLoader) Full(context.Context) ([]domainsuggest.ProfileSearchTerm, error) {
	l.fullCalls++
	close(l.entered)
	<-l.release
	return nil, nil
}

func (l *blockingSuggestLoader) Delta(context.Context, time.Time) ([]domainsuggest.ProfileIndexMutation, error) {
	l.deltaCalls++
	return nil, nil
}

type recordingSuggestMetrics struct {
	kind       string
	result     string
	upserts    int
	tombstones int
}

func (m *recordingSuggestMetrics) RecordQuery(string, int, bool)  {}
func (m *recordingSuggestMetrics) ObserveRefresh(string, float64) {}
func (m *recordingSuggestMetrics) RecordRateLimited(bool)         {}
func (m *recordingSuggestMetrics) RecordRefresh(kind, result string, upserts, tombstones int, _ time.Time) {
	m.kind = kind
	m.result = result
	m.upserts = upserts
	m.tombstones = tombstones
}

func TestProfileIndexRefresherCountsMutationItems(t *testing.T) {
	upserts, tombstones := countMutationItems([]domainsuggest.ProfileIndexMutation{
		mustUpsert(domainsuggest.NewProfileSearchTerm(1, "a", nil, 1, 0, nil)),
		mustDelete(2),
		mustUpsert(domainsuggest.NewProfileSearchTerm(3, "c", nil, 1, 0, nil)),
	})
	if upserts != 2 || tombstones != 1 {
		t.Fatalf("upserts=%d tombstones=%d, want 2/1", upserts, tombstones)
	}
}

func mustDelete(id int64) domainsuggest.ProfileIndexMutation {
	m, err := domainsuggest.NewProfileIndexDelete(id)
	if err != nil {
		panic(err)
	}
	return m
}

func TestProfileIndexRefresherFullMetricsRecordUpsertAndTombstone(t *testing.T) {
	metrics := &recordingSuggestMetrics{}
	runtime := &suggestRuntimeStub{}
	loader := &suggestLoaderStub{
		full: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "a", nil, 1, 0, nil),
			domainsuggest.NewProfileSearchTerm(2, "", nil, 1, 0, nil),
		},
	}
	refresher := NewProfileIndexRefresher(loader, runtime, metrics)
	if err := refresher.RunFull(context.Background()); err != nil {
		t.Fatal(err)
	}
	if metrics.kind != "full" || metrics.result != "success" || metrics.upserts != 1 || metrics.tombstones != 1 {
		t.Fatalf("metrics = (%s,%s,%d,%d)", metrics.kind, metrics.result, metrics.upserts, metrics.tombstones)
	}
}

func TestProfileIndexRefresherDeltaBeforeFirstFullIsNoOp(t *testing.T) {
	refresher := NewProfileIndexRefresher(&suggestLoaderStub{}, &suggestRuntimeStub{}, nil)
	if err := refresher.RunDelta(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !refresher.lastFetch.IsZero() {
		t.Fatalf("lastFetch advanced without full: %v", refresher.lastFetch)
	}
}
