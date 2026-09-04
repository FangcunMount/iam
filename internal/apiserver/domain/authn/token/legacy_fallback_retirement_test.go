package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLegacyFallbackRetirementPolicyUsesLongestConfiguredLifetime(t *testing.T) {
	t.Parallel()

	policy := NewLegacyFallbackRetirementPolicy(15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	window, err := policy.RequiredZeroWindow()
	require.NoError(t, err)
	require.Equal(t, 7*24*time.Hour, window)
}

func TestLegacyFallbackRetirementRequiresUpgradedFleetAndQuietMetricsForFullWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	policy := NewLegacyFallbackRetirementPolicy(time.Hour, 24*time.Hour, 12*time.Hour)

	canRetire, err := policy.CanRetireAt(now, LegacyFallbackObservation{
		AllInstancesCurrentSince:    now.Add(-48 * time.Hour),
		AllFallbackMetricsZeroSince: now.Add(-23 * time.Hour),
	})
	require.NoError(t, err)
	require.False(t, canRetire)

	canRetire, err = policy.CanRetireAt(now, LegacyFallbackObservation{
		AllInstancesCurrentSince:    now.Add(-48 * time.Hour),
		AllFallbackMetricsZeroSince: now.Add(-24 * time.Hour),
	})
	require.NoError(t, err)
	require.True(t, canRetire)
}

func TestLegacyFallbackRetirementRejectsInvalidTTLConfiguration(t *testing.T) {
	t.Parallel()

	policy := NewLegacyFallbackRetirementPolicy(0, time.Hour, time.Hour)
	_, err := policy.RequiredZeroWindow()
	require.Error(t, err)
}
