package credential

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestCredential_TypeChecksAndFactories(t *testing.T) {
	loginIdentityID := meta.FromUint64(1)
	pwd := NewPasswordCredential(loginIdentityID, []byte("hash"), "argon2id")
	require.NotNil(t, pwd)
	assert.Equal(t, loginIdentityID, pwd.LoginIdentityID)
	assert.True(t, pwd.IsPasswordType())
	assert.True(t, pwd.IsEnabled())
}

func TestCredential_SuccessFailureAndLocking(t *testing.T) {
	now := time.Now()
	c := NewPasswordCredential(meta.FromUint64(2), []byte("h"), "bcrypt")

	// initial
	assert.True(t, c.IsEnabled())
	assert.False(t, c.IsLockedByTime(now))

	// record failures
	cnt := c.RecordFailure(now)
	assert.Equal(t, 1, cnt)
	assert.NotNil(t, c.LastFailureAt)

	// should lock when threshold reached
	c.FailedAttempts = 3
	locked := c.ShouldLock(3)
	assert.True(t, locked)

	// ApplyLockPolicy
	policy := LockoutPolicy{Enabled: true, Threshold: 3, LockDuration: 2 * time.Hour}
	applied := c.ApplyLockPolicy(now, policy)
	assert.True(t, applied)
	assert.NotNil(t, c.LockedUntil)
	assert.True(t, c.IsLockedByTime(now))

	// Unlock
	c.Unlock()
	assert.False(t, c.IsLockedByTime(now))
	assert.Equal(t, 0, c.FailedAttempts)

	// Record success resets attempts
	c.RecordFailure(now)
	c.RecordSuccess(now)
	assert.Equal(t, 0, c.FailedAttempts)
	require.NotNil(t, c.LastSuccessAt)
}

func TestCredential_UpdateAndRotate(t *testing.T) {
	c := NewPasswordCredential(meta.FromUint64(3), []byte("old"), "oldalg")
	newMat := []byte("new")
	newAlg := "argon2id"
	c.RotateMaterial(newMat, &newAlg)
	assert.Equal(t, newMat, c.Material)
	assert.Equal(t, &newAlg, c.Algo)
}
