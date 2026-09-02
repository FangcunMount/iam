package credential

import (
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestApplyAuthenticationTransitionFailureAndSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	cred := NewPasswordCredential(meta.FromUint64(10), []byte("old"), "argon2id")
	policy := LockoutPolicy{Enabled: true, Threshold: 2, LockDuration: time.Hour}

	state := ApplyAuthenticationTransition(cred, NewFailureTransition(meta.FromUint64(1), now, policy))
	require.Equal(t, 1, state.FailedAttempts)
	require.False(t, state.NewlyLocked)

	state = ApplyAuthenticationTransition(cred, NewFailureTransition(meta.FromUint64(1), now, policy))
	require.Equal(t, 2, state.FailedAttempts)
	require.True(t, state.NewlyLocked)
	require.NotNil(t, state.LockedUntil)

	rotation := &MaterialRotation{Material: []byte("new"), Algo: strPtr("argon2id")}
	ApplyAuthenticationTransition(cred, NewSuccessTransition(meta.FromUint64(1), now, rotation))
	require.Equal(t, 0, cred.FailedAttempts)
	require.Equal(t, []byte("new"), cred.Material)
}

func TestApplyAuthenticationTransitionIgnoresNoneKind(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	cred := NewPasswordCredential(meta.FromUint64(10), []byte("old"), "argon2id")
	cred.RecordFailure(now)

	state := ApplyAuthenticationTransition(cred, AuthenticationTransition{Kind: TransitionNone})
	require.Equal(t, AuthenticationState{}, state)
	require.Equal(t, 1, cred.FailedAttempts)
}

func strPtr(v string) *string {
	return &v
}
