package credential_test

import (
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestBinder_PasswordValidation(t *testing.T) {
	b := credential.NewBinder()

	// missing account id
	_, err := b.Bind(credential.BindSpec{LoginIdentityID: 0, Type: credential.CredPassword})
	require.Error(t, err)

	// missing type
	_, err = b.Bind(credential.BindSpec{LoginIdentityID: meta.ID(1), Type: ""})
	require.Error(t, err)

	// password requires material and algo
	_, err = b.Bind(credential.BindSpec{LoginIdentityID: meta.ID(1), Type: credential.CredPassword, Material: nil})
	require.Error(t, err)
	algo := "argon2id"
	_, err = b.Bind(credential.BindSpec{LoginIdentityID: meta.ID(1), Type: credential.CredPassword, Material: []byte("m"), Algo: &algo})
	require.NoError(t, err)
}

func TestBinder_RejectsNonPersistentCredentialTypes(t *testing.T) {
	b := credential.NewBinder()

	_, err := b.Bind(credential.BindSpec{LoginIdentityID: meta.ID(1), Type: credential.CredPhoneOTP})
	require.Error(t, err)

	_, err = b.Bind(credential.BindSpec{LoginIdentityID: meta.ID(1), Type: credential.CredOAuthWxMinip})
	require.Error(t, err)

	_, err = b.Bind(credential.BindSpec{LoginIdentityID: meta.ID(1), Type: "unknown"})
	require.Error(t, err)
}

func TestRotator_RotateBehavior(t *testing.T) {
	r := credential.NewRotator()

	// nil credential: should be a no-op (no panic)
	r.Rotate(nil, []byte("x"), nil)

	// disabled credential: Rotate still delegates to RotateMaterial (no validation here)
	c := &credential.Credential{ID: meta.ID(1), LoginIdentityID: meta.ID(2)}
	c.Disable()
	// rotating with empty newMaterial is a no-op
	r.Rotate(c, []byte{}, nil)
	require.Nil(t, c.Algo)

	// happy path: rotate with material and algo
	c.Enable()
	algo := "bcrypt"
	r.Rotate(c, []byte("new2"), &algo)
	require.Equal(t, []byte("new2"), c.Material)
	require.Equal(t, &algo, c.Algo)
}

func TestCredential_RecordAndLockPolicy(t *testing.T) {
	now := time.Now()
	c := credential.NewPasswordCredential(meta.ID(9), []byte("h"), "a")

	// record failure increments
	cnt := c.RecordFailure(now)
	require.Equal(t, 1, cnt)
	cnt = c.RecordFailure(now)
	require.Equal(t, 2, cnt)

	// ShouldLock
	require.True(t, c.ShouldLock(2))

	// ApplyLockPolicy locks when threshold reached
	policy := credential.LockoutPolicy{Enabled: true, Threshold: 2, LockDuration: time.Hour}
	locked := c.ApplyLockPolicy(now, policy)
	require.True(t, locked)
	require.True(t, c.IsLockedByTime(now.Add(30*time.Minute)))

	// Unlock resets
	c.Unlock()
	require.False(t, c.IsLockedByTime(now))

	// Record success resets failures
	c.RecordFailure(now)
	c.RecordSuccess(now)
	require.Equal(t, 0, c.FailedAttempts)
}
