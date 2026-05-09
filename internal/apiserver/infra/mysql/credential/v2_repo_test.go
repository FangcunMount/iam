package credential

import (
	"context"
	"testing"
	"time"

	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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

	credentialID, material, err := repo.FindPasswordCredentialByLoginIdentity(context.Background(), loginIdentityID)
	require.NoError(t, err)
	require.Equal(t, found.ID, credentialID)
	require.Equal(t, "hash", material)
}

func TestCredentialRepositoryUpdatesV2PasswordAuthState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&V2PO{}))

	repo := NewRepository(db)
	loginIdentityID := meta.FromUint64(2001)
	cred := credDomain.NewPasswordCredential(loginIdentityID, []byte("hash"), "argon2id")
	require.NoError(t, repo.Create(ctx, cred))

	lockedUntil := time.Now().Add(time.Minute).Truncate(time.Second)
	lastSuccessAt := time.Now().Add(2 * time.Minute).Truncate(time.Second)
	lastFailureAt := time.Now().Add(3 * time.Minute).Truncate(time.Second)
	require.NoError(t, repo.UpdateStatus(ctx, cred.ID, credDomain.CredStatusDisabled))
	require.NoError(t, repo.UpdateFailedAttempts(ctx, cred.ID, 3))
	require.NoError(t, repo.UpdateLockedUntil(ctx, cred.ID, &lockedUntil))
	require.NoError(t, repo.UpdateLastSuccessAt(ctx, cred.ID, lastSuccessAt))
	require.NoError(t, repo.UpdateLastFailureAt(ctx, cred.ID, lastFailureAt))

	found, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, credDomain.CredStatusDisabled, found.Status)
	require.Equal(t, 3, found.FailedAttempts)
	require.NotNil(t, found.LockedUntil)
	require.True(t, found.LockedUntil.Equal(lockedUntil))
	require.NotNil(t, found.LastSuccessAt)
	require.True(t, found.LastSuccessAt.Equal(lastSuccessAt))
	require.NotNil(t, found.LastFailureAt)
	require.True(t, found.LastFailureAt.Equal(lastFailureAt))
}
