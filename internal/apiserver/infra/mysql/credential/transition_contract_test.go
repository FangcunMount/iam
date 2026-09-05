package credential_test

import (
	"context"
	"testing"
	"time"

	testutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/testutil"
	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	mysqlcred "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/credential"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRepositoryApplyAuthenticationTransitionMatchesEntitySemantics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.OpenDBForIntegrationTest(t, &mysqlcred.V2PO{})

	repo := mysqlcred.NewRepository(db)
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	policy := credDomain.LockoutPolicy{Enabled: true, Threshold: 3, LockDuration: 2 * time.Hour}

	entity := credDomain.NewPasswordCredential(meta.FromUint64(uint64(time.Now().UnixNano())), []byte("hash"), "argon2id")
	require.NoError(t, repo.Create(ctx, entity))
	t.Cleanup(func() { db.Unscoped().Where("id = ?", entity.ID).Delete(&mysqlcred.V2PO{}) })

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
		if entityStates[i].LockedUntil == nil {
			require.Nil(t, repoState.LockedUntil, "must not lock before reaching the threshold")
		} else {
			require.NotNil(t, repoState.LockedUntil)
			require.WithinDuration(t, *entityStates[i].LockedUntil, *repoState.LockedUntil, time.Millisecond)
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
