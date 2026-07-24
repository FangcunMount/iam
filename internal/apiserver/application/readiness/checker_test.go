package readiness

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckerRequiredAndOptionalComponents(t *testing.T) {
	t.Parallel()
	checker := New(Config{ComponentTimeout: 20 * time.Millisecond, TotalTimeout: 100 * time.Millisecond}, []Component{
		{Name: "mysql", Required: true, Check: func(context.Context) error { return nil }},
		{Name: "suggest", Required: false, Check: func(context.Context) error { return errors.New("unavailable") }},
	})
	snapshot, ready := checker.Check(context.Background())
	require.True(t, ready)
	require.Equal(t, StatusOK, snapshot.Components["mysql"].Status)
	require.Equal(t, StatusDegraded, snapshot.Components["suggest"].Status)

	checker.MarkDraining()
	snapshot, ready = checker.Check(context.Background())
	require.False(t, ready)
	require.Equal(t, "not_ready", snapshot.Status)
}

func TestCheckerTimesOutRequiredComponentWithoutExposingError(t *testing.T) {
	t.Parallel()
	checker := New(Config{ComponentTimeout: time.Millisecond, TotalTimeout: 20 * time.Millisecond}, []Component{{
		Name: "redis", Required: true, Check: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}})
	snapshot, ready := checker.Check(context.Background())
	require.False(t, ready)
	require.Equal(t, StatusFailed, snapshot.Components["redis"].Status)
}
