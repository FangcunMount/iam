package refreshindex

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
)

func TestRefresherFullUsesQueryStartAsDeltaCursor(t *testing.T) {
	queryStart := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	refresher := NewRefresher(&loaderStub{}, &writerStub{}, nil)
	refresher.now = func() time.Time { return queryStart }

	if err := refresher.RunFull(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !refresher.lastFetch.Equal(queryStart) {
		t.Fatalf("lastFetch = %s, want query start %s", refresher.lastFetch, queryStart)
	}
	if !refresher.HasSuccessfulRefresh() {
		t.Fatal("HasSuccessfulRefresh = false after successful full")
	}
}

func TestRefresherEmptyDeltaAdvancesCursor(t *testing.T) {
	previous := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	windowStart := previous.Add(time.Minute)
	loader := &recordingLoader{}
	refresher := NewRefresher(loader, &writerStub{}, nil)
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

func TestRefresherFailureDoesNotAdvanceCursor(t *testing.T) {
	previous := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	wantErr := errors.New("apply failed")
	upsert, err := Upsert(profile.MustNew(1, "profile", nil, 1, 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	loader := &recordingLoader{delta: []ProjectionChange{upsert}}
	refresher := NewRefresher(loader, &writerStub{applyErr: wantErr}, nil)
	refresher.lastFetch = previous
	refresher.now = func() time.Time { return previous.Add(time.Minute) }

	if err := refresher.RunDelta(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("RunDelta() error = %v, want %v", err, wantErr)
	}
	if !refresher.lastFetch.Equal(previous) {
		t.Fatalf("lastFetch advanced to %s after failure; want %s", refresher.lastFetch, previous)
	}
}

func TestRefresherSourceFailureDoesNotAdvanceCursor(t *testing.T) {
	previous := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	wantErr := errors.New("delta query failed")
	loader := &recordingLoader{deltaErr: wantErr}
	refresher := NewRefresher(loader, &writerStub{}, nil)
	refresher.lastFetch = previous
	refresher.now = func() time.Time { return previous.Add(time.Minute) }

	if err := refresher.RunDelta(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("RunDelta() error = %v, want %v", err, wantErr)
	}
	if !refresher.lastFetch.Equal(previous) {
		t.Fatal("lastFetch advanced after source failure")
	}
}

func TestRefresherRejectsOverlappingRefreshes(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	loader := &blockingLoader{entered: entered, release: release}
	refresher := NewRefresher(loader, &writerStub{}, nil)
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

func TestRefresherFullMetricsRecordUpsertAndTombstone(t *testing.T) {
	metrics := &recordingMetrics{}
	writer := &writerStub{}
	loader := &loaderStub{
		full: []profile.SuggestibleProfile{
			profile.MustNew(1, "a", nil, 1, 0, nil),
			profile.RawProjection(2, "", nil, 1, 0, nil),
		},
	}
	refresher := NewRefresher(loader, writer, metrics)
	if err := refresher.RunFull(context.Background()); err != nil {
		t.Fatal(err)
	}
	if metrics.kind != "full" || metrics.result != "success" || metrics.upserts != 1 || metrics.tombstones != 1 {
		t.Fatalf("metrics = (%s,%s,%d,%d)", metrics.kind, metrics.result, metrics.upserts, metrics.tombstones)
	}
}

func TestRefresherDeltaBeforeFirstFullIsNoOp(t *testing.T) {
	refresher := NewRefresher(&loaderStub{}, &writerStub{}, nil)
	if err := refresher.RunDelta(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !refresher.lastFetch.IsZero() {
		t.Fatalf("lastFetch advanced without full: %v", refresher.lastFetch)
	}
	if refresher.HasSuccessfulRefresh() {
		t.Fatal("HasSuccessfulRefresh = true before any successful refresh")
	}
}

func TestRefresherFullWithoutWriterFails(t *testing.T) {
	previous := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	refresher := NewRefresher(&loaderStub{
		full: []profile.SuggestibleProfile{profile.MustNew(1, "a", nil, 1, 0, nil)},
	}, nil, nil)
	refresher.lastFetch = previous

	err := refresher.RunFull(context.Background())
	if err == nil || err.Error() != "suggest store not initialized" {
		t.Fatalf("RunFull() error = %v", err)
	}
	if !refresher.lastFetch.Equal(previous) {
		t.Fatal("lastFetch advanced after writer failure")
	}
	if refresher.HasSuccessfulRefresh() {
		t.Fatal("HasSuccessfulRefresh = true after writer failure")
	}
}

func TestRefresherOverlapRecordsRefreshInProgress(t *testing.T) {
	metrics := &recordingMetrics{}
	entered := make(chan struct{})
	release := make(chan struct{})
	loader := &blockingLoader{entered: entered, release: release}
	refresher := NewRefresher(loader, &writerStub{}, metrics)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = refresher.RunFull(context.Background())
	}()
	<-entered

	if err := refresher.RunFull(context.Background()); !errors.Is(err, ErrRefreshInProgress) {
		t.Fatalf("RunFull() error = %v, want %v", err, ErrRefreshInProgress)
	}
	if metrics.result != "refresh_in_progress" {
		t.Fatalf("metrics result = %q, want refresh_in_progress", metrics.result)
	}
	close(release)
	wg.Wait()
}

type loaderStub struct {
	full []profile.SuggestibleProfile
}

func (l *loaderStub) Full(context.Context) ([]profile.SuggestibleProfile, error) {
	return append([]profile.SuggestibleProfile(nil), l.full...), nil
}

func (l *loaderStub) Delta(context.Context, time.Time) ([]ProjectionChange, error) {
	return nil, nil
}

type recordingLoader struct {
	since    time.Time
	delta    []ProjectionChange
	deltaErr error
}

func (l *recordingLoader) Full(context.Context) ([]profile.SuggestibleProfile, error) {
	return nil, nil
}

func (l *recordingLoader) Delta(_ context.Context, since time.Time) ([]ProjectionChange, error) {
	l.since = since
	if l.deltaErr != nil {
		return nil, l.deltaErr
	}
	return append([]ProjectionChange(nil), l.delta...), nil
}

type blockingLoader struct {
	entered chan struct{}
	release chan struct{}

	fullCalls  int
	deltaCalls int
}

func (l *blockingLoader) Full(context.Context) ([]profile.SuggestibleProfile, error) {
	l.fullCalls++
	close(l.entered)
	<-l.release
	return nil, nil
}

func (l *blockingLoader) Delta(context.Context, time.Time) ([]ProjectionChange, error) {
	l.deltaCalls++
	return nil, nil
}

type writerStub struct {
	replaced []profile.SuggestibleProfile
	applied  []ProjectionChange
	applyErr error
}

func (w *writerStub) Replace(_ context.Context, profiles []profile.SuggestibleProfile) error {
	w.replaced = append([]profile.SuggestibleProfile(nil), profiles...)
	return nil
}

func (w *writerStub) Apply(_ context.Context, changes []ProjectionChange) error {
	if w.applyErr != nil {
		return w.applyErr
	}
	w.applied = append([]ProjectionChange(nil), changes...)
	return nil
}

type recordingMetrics struct {
	kind       string
	result     string
	upserts    int
	tombstones int
}

func (m *recordingMetrics) ObserveRefresh(string, float64) {}
func (m *recordingMetrics) RecordRefresh(kind, result string, upserts, tombstones int, _ time.Time) {
	m.kind = kind
	m.result = result
	m.upserts = upserts
	m.tombstones = tombstones
}
