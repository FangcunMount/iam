package guardianship

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	testhelpers "github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_Create_DuplicateReturnsBusinessError(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	// Ensure table exists with unique index defined in PO tags
	require.NoError(t, db.AutoMigrate(&GuardianshipPO{}))

	repo := NewRepository(db)
	ctx := context.Background()

	userID1 := meta.FromUint64(1)
	childID2 := meta.FromUint64(2)
	g1 := &guardianship.Guardianship{
		User:  userID1,
		Child: childID2,
		Rel:   guardianship.RelParent,
	}

	// first create should succeed
	err := repo.Create(ctx, g1)
	require.NoError(t, err)

	// second create with same user+child should be treated as business 'exists' error
	userID1_2 := meta.FromUint64(1)
	childID2_2 := meta.FromUint64(2)
	g2 := &guardianship.Guardianship{
		User:  userID1_2,
		Child: childID2_2,
		Rel:   guardianship.RelParent,
	}
	err = repo.Create(ctx, g2)
	require.Error(t, err)

	// We expect the error to be wrapped with the registered business code
	require.True(t, perrors.IsCode(err, code.ErrIdentityGuardianshipExists), "error must be mapped to ErrIdentityGuardianshipExists")
}

func TestRepository_DefaultQueriesExcludeRevokedGuardianships(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&GuardianshipPO{}))

	repo := NewRepository(db)
	ctx := context.Background()

	record := &guardianship.Guardianship{
		User:          meta.FromUint64(1001),
		Child:         meta.FromUint64(2002),
		Rel:           guardianship.RelParent,
		EstablishedAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, repo.Create(ctx, record))

	record.Revoke(time.Now())
	require.NoError(t, repo.Update(ctx, record))

	_, err := repo.FindByUserIDAndChildID(ctx, record.User, record.Child)
	require.Error(t, err)

	historical, err := repo.FindByUserIDAndChildIDIncludingRevoked(ctx, record.User, record.Child)
	require.NoError(t, err)
	require.NotNil(t, historical)
	require.NotNil(t, historical.RevokedAt)

	activeByUser, err := repo.FindByUserID(ctx, record.User)
	require.NoError(t, err)
	assert.Len(t, activeByUser, 0)

	withRevokedByUser, err := repo.FindByUserIDIncludingRevoked(ctx, record.User)
	require.NoError(t, err)
	assert.Len(t, withRevokedByUser, 1)

	activeByChild, err := repo.FindByChildID(ctx, record.Child)
	require.NoError(t, err)
	assert.Len(t, activeByChild, 0)

	withRevokedByChild, err := repo.FindByChildIDIncludingRevoked(ctx, record.Child)
	require.NoError(t, err)
	assert.Len(t, withRevokedByChild, 1)

	isGuardian, err := repo.IsGuardian(ctx, record.User, record.Child)
	require.NoError(t, err)
	assert.False(t, isGuardian)
}
