package session

import (
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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
