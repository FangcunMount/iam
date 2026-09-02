package credential_test

import (
	"context"
	"testing"
	"time"

	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	mysqlcred "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/credential"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRepositoryApplyAuthenticationTransitionMatchesEntitySemantics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&mysqlcred.V2PO{}))

	repo := mysqlcred.NewRepository(db)
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	policy := credDomain.LockoutPolicy{Enabled: true, Threshold: 3, LockDuration: 2 * time.Hour}

	entity := credDomain.NewPasswordCredential(meta.FromUint64(2002), []byte("hash"), "argon2id")
	require.NoError(t, repo.Create(ctx, entity))

	var entityStates []credDomain.AuthenticationState
	for i := 0; i < 3; i++ {
		entityStates = append(entityStates, credDomain.ApplyAuthenticationTransition(
			entity,
			credDomain.NewFailureTransition(entity.ID, now, policy),
		))
		repoState, err := repo.ApplyAuthenticationTransition(ctx, credDomain.NewFailureTransition(entity.ID, now, policy))
		require.NoError(t, err)
		require.Equal(t, entityStates[i].FailedAttempts, repoState.FailedAttempts)
		require.Equal(t, entityStates[i].NewlyLocked, repoState.NewlyLocked)
		if entityStates[i].LockedUntil != nil {
			require.NotNil(t, repoState.LockedUntil)
		}
	}

	rotation := &credDomain.MaterialRotation{Material: []byte("new-hash"), Algo: strPtr("argon2id-v2")}
	credDomain.ApplyAuthenticationTransition(entity, credDomain.NewSuccessTransition(entity.ID, now, rotation))
	_, err := repo.ApplyAuthenticationTransition(ctx, credDomain.NewSuccessTransition(entity.ID, now, rotation))
	require.NoError(t, err)

	found, err := repo.GetByID(ctx, entity.ID)
	require.NoError(t, err)
	require.Equal(t, 0, found.FailedAttempts)
	require.Equal(t, []byte("new-hash"), found.Material)
}

func strPtr(v string) *string {
	return &v
}
