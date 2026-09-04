package signingkey

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatusTransitionsAndCapabilities(t *testing.T) {
	t.Parallel()

	require.True(t, StatusActive.CanSign())
	require.True(t, StatusActive.CanVerify())
	require.False(t, StatusGrace.CanSign())
	require.True(t, StatusGrace.CanVerify())
	require.False(t, StatusRetired.CanVerify())

	grace, err := StatusActive.EnterGrace()
	require.NoError(t, err)
	require.Equal(t, StatusGrace, grace)
	retired, err := grace.Retire()
	require.NoError(t, err)
	require.Equal(t, StatusRetired, retired)
	_, err = StatusActive.Retire()
	require.Error(t, err)
}

func TestRotationPolicyValidation(t *testing.T) {
	t.Parallel()

	require.NoError(t, DefaultRotationPolicy().Validate())
	require.Error(t, (RotationPolicy{RotationInterval: time.Hour, GracePeriod: time.Hour, MaxPublishableKeys: 2}).Validate())
}

func TestKeyOwnsValidityCapabilitiesAndLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	graceUntil := now.Add(time.Hour)
	key := NewKey(
		"kid-1",
		"RS256",
		WithNotBefore(now.Add(-time.Minute)),
		WithNotAfter(now.Add(24*time.Hour)),
	)
	require.NoError(t, key.Validate())
	require.True(t, key.CanSignAt(now))
	require.True(t, key.CanVerifyAt(now))
	require.True(t, key.ShouldPublishAt(now))

	require.NoError(t, key.EnterGrace(graceUntil, now))
	require.False(t, key.CanSignAt(now))
	require.True(t, key.CanVerifyAt(now))
	require.Error(t, key.Retire(now))

	require.NoError(t, key.Retire(graceUntil))
	require.False(t, key.CanVerifyAt(graceUntil))
	require.False(t, key.ShouldPublishAt(graceUntil))
}

func TestKeyRejectsInvalidIdentityAndTimeRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	require.Error(t, NewKey("", "RS256").Validate())
	require.Error(t, NewKey("kid-1", "").Validate())
	require.Error(t, NewKey(
		"kid-1",
		"RS256",
		WithNotBefore(now),
		WithNotAfter(now),
	).Validate())
}
