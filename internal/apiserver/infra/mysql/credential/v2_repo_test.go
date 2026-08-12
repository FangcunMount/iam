package credential

import (
	"context"
	"testing"
	"time"

	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCredentialRepositoryCreatesAndFindsV2PasswordCredential(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&V2PO{}))

	repo := NewRepository(db)
	loginIdentityID := meta.FromUint64(2001)
	cred := credDomain.NewPasswordCredential(loginIdentityID, []byte("hash"), "argon2id")

	require.NoError(t, repo.Create(context.Background(), cred))
	require.False(t, cred.ID.IsZero())

	found, err := repo.GetByLoginIdentityIDAndType(context.Background(), loginIdentityID, credDomain.CredPassword)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, loginIdentityID, found.LoginIdentityID)
	require.Equal(t, credDomain.CredPassword, found.Type)

	passwordRecord, err := repo.FindPasswordCredentialByLoginIdentity(context.Background(), loginIdentityID)
	require.NoError(t, err)
	require.NotNil(t, passwordRecord)
	require.Equal(t, found.ID, passwordRecord.CredentialID)
	require.Equal(t, "hash", passwordRecord.PasswordHash)
	require.Equal(t, credDomain.CredStatusEnabled, passwordRecord.Status)
}

func TestCredentialRepositoryUpdatesStatusAndRecordsFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&V2PO{}))

	repo := NewRepository(db)
	loginIdentityID := meta.FromUint64(2001)
	cred := credDomain.NewPasswordCredential(loginIdentityID, []byte("hash"), "argon2id")
	require.NoError(t, repo.Create(ctx, cred))

	cred.Status = credDomain.CredStatusDisabled

	require.NoError(t, repo.UpdateStatus(ctx, cred.ID, cred.Status))
	now := time.Now().UTC().Truncate(time.Second)
	state, err := repo.RecordAuthenticationFailure(ctx, cred.ID, now, credDomain.LockoutPolicy{})
	require.NoError(t, err)
	require.Equal(t, 1, state.FailedAttempts)

	found, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, credDomain.CredStatusDisabled, found.Status)
	require.Equal(t, 1, found.FailedAttempts)
	require.Nil(t, found.LockedUntil)
	require.Nil(t, found.LastSuccessAt)
	require.NotNil(t, found.LastFailureAt)
	require.True(t, found.LastFailureAt.Equal(now))
}

func TestCredentialRepositoryRecordsAuthenticationSuccessAtomically(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&V2PO{}))
	repo := NewRepository(db)
	cred := credDomain.NewPasswordCredential(meta.FromUint64(2001), []byte("old-hash"), "argon2id")
	require.NoError(t, repo.Create(ctx, cred))
	for range 4 {
		_, err := repo.RecordAuthenticationFailure(ctx, cred.ID, time.Now(), credDomain.LockoutPolicy{})
		require.NoError(t, err)
	}

	newAlgo := "argon2id-v2"
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.RecordAuthenticationSuccess(ctx, cred.ID, now, &credDomain.MaterialRotation{
		Material: []byte("new-hash"),
		Algo:     &newAlgo,
	}))

	found, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	require.Equal(t, 0, found.FailedAttempts)
	require.Equal(t, []byte("new-hash"), found.Material)
	require.NotNil(t, found.Algo)
	require.Equal(t, newAlgo, *found.Algo)
	require.NotNil(t, found.LastSuccessAt)
	require.True(t, found.LastSuccessAt.Equal(now))
}
