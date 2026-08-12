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
		delta: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "profile", nil, 1, 0, nil),
		},
	}
	refresher := NewProfileIndexRefresher(loader, &suggestRuntimeStub{importErr: wantErr}, nil)
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
	delta []domainsuggest.ProfileSearchTerm
}

func (l *recordingSuggestLoader) Full(context.Context) ([]domainsuggest.ProfileSearchTerm, error) {
	return nil, nil
}

func (l *recordingSuggestLoader) Delta(_ context.Context, since time.Time) ([]domainsuggest.ProfileSearchTerm, error) {
	l.since = since
	return append([]domainsuggest.ProfileSearchTerm(nil), l.delta...), nil
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

func (l *blockingSuggestLoader) Delta(context.Context, time.Time) ([]domainsuggest.ProfileSearchTerm, error) {
	l.deltaCalls++
	return nil, nil
}
