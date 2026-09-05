package session

import (
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestLifetimePolicyCapsInitialExpiryBySessionMaxTTL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	policy := NewLifetimePolicy(7*24*time.Hour, 24*time.Hour)

	expiresAt, err := policy.InitialExpiresAt(now)

	require.NoError(t, err)
	require.Equal(t, now.Add(24*time.Hour), expiresAt)
}

func TestLifetimePolicyRejectsSessionPastMaximumLifetime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	policy := NewLifetimePolicy(7*24*time.Hour, 24*time.Hour)
	sess := New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, now.Add(7*24*time.Hour))
	sess.CreatedAt = now.Add(-25 * time.Hour)

	err := policy.EnsureActiveWithinLifetime(now, sess)

	require.Error(t, err)
	require.Equal(t, code.ErrSessionInactive, perrors.ParseCoder(err).Code())
}

func TestLifetimePolicyCapsExtensionByMaximumLifetime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	policy := NewLifetimePolicy(7*24*time.Hour, 24*time.Hour)
	sess := New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, now.Add(7*24*time.Hour))
	sess.CreatedAt = now.Add(-23 * time.Hour)

	expiresAt, err := policy.ExtensionExpiresAt(now, sess, now.Add(7*24*time.Hour))

	require.NoError(t, err)
	require.Equal(t, sess.CreatedAt.Add(24*time.Hour), expiresAt)
}

func TestLifetimePolicySlidesRefreshWindowWithinAbsoluteLimit(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	policy := NewLifetimePolicy(time.Hour, 2*time.Hour)
	expiry, err := policy.InitialExpiresAt(created)
	require.NoError(t, err)
	sess := &Session{CreatedAt: created, ExpiresAt: expiry, Status: StatusActive}
	for _, tc := range []struct{ elapsed, want time.Duration }{
		{30 * time.Minute, 90 * time.Minute},
		{75 * time.Minute, 120 * time.Minute},
	} {
		now := created.Add(tc.elapsed)
		refreshExpiry, err := policy.RefreshTokenExpiresAt(now, sess)
		require.NoError(t, err)
		require.Equal(t, created.Add(tc.want), refreshExpiry)
		sess.ExpiresAt, err = policy.ExtensionExpiresAt(now, sess, refreshExpiry)
		require.NoError(t, err)
		require.Equal(t, refreshExpiry, sess.ExpiresAt)
	}
	_, err = policy.RefreshTokenExpiresAt(created.Add(2*time.Hour), sess)
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrSessionInactive))
}

func TestLifetimePolicyPreservesLegacyExpiryWithoutKnownAbsoluteLimit(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	sess := &Session{ExpiresAt: now.Add(30 * time.Minute)}
	for _, maximum := range []time.Duration{0, 24 * time.Hour} {
		expiry, err := NewLifetimePolicy(time.Hour, maximum).RefreshTokenExpiresAt(now, sess)
		require.NoError(t, err)
		require.Equal(t, sess.ExpiresAt, expiry)
	}
}
