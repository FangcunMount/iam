package profilelink

import (
	"context"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profilelink"
	testhelpers "github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_Create_DuplicateReturnsBusinessError(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	// Ensure table exists with unique index defined in PO tags
	require.NoError(t, db.AutoMigrate(&ProfileLinkPO{}))

	repo := NewRepository(db)
	ctx := context.Background()

	userID1 := meta.FromUint64(1)
	profileID2 := meta.FromUint64(2)
	g1 := &profilelink.ProfileLink{
		User:    userID1,
		Profile: profileID2,
		Rel:     profilelink.RelParent,
	}

	// first create should succeed
	err := repo.Create(ctx, g1)
	require.NoError(t, err)

	// second create with same user+profile should be treated as business 'exists' error
	userID1_2 := meta.FromUint64(1)
	profileID2_2 := meta.FromUint64(2)
	g2 := &profilelink.ProfileLink{
		User:    userID1_2,
		Profile: profileID2_2,
		Rel:     profilelink.RelParent,
	}
	err = repo.Create(ctx, g2)
	require.Error(t, err)

	// We expect the error to be wrapped with the registered business code
	require.True(t, perrors.IsCode(err, code.ErrIdentityProfileLinkExists), "error must be mapped to ErrIdentityProfileLinkExists")
}

func TestRepository_DefaultQueriesExcludeRevokedRefs(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&ProfileLinkPO{}))

	repo := NewRepository(db)
	ctx := context.Background()

	record := &profilelink.ProfileLink{
		User:          meta.FromUint64(1001),
		Profile:       meta.FromUint64(2002),
		Rel:           profilelink.RelParent,
		EstablishedAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, repo.Create(ctx, record))

	record.Revoke(time.Now())
	require.NoError(t, repo.Update(ctx, record))

	_, err := repo.FindByUserIDAndProfileID(ctx, record.User, record.Profile)
	require.Error(t, err)

	historical, err := repo.FindByUserIDAndProfileIDIncludingRevoked(ctx, record.User, record.Profile)
	require.NoError(t, err)
	require.NotNil(t, historical)
	require.NotNil(t, historical.RevokedAt)

	activeByUser, err := repo.FindByUserID(ctx, record.User)
	require.NoError(t, err)
	assert.Len(t, activeByUser, 0)

	withRevokedByUser, err := repo.FindByUserIDIncludingRevoked(ctx, record.User)
	require.NoError(t, err)
	assert.Len(t, withRevokedByUser, 1)

	activeByProfile, err := repo.FindByProfileID(ctx, record.Profile)
	require.NoError(t, err)
	assert.Len(t, activeByProfile, 0)

	withRevokedByProfile, err := repo.FindByProfileIDIncludingRevoked(ctx, record.Profile)
	require.NoError(t, err)
	assert.Len(t, withRevokedByProfile, 1)

	hasProfileLink, err := repo.IsLinked(ctx, record.User, record.Profile)
	require.NoError(t, err)
	assert.False(t, hasProfileLink)
}

func TestRepository_CreateConcurrentSelfLinksAllowsOnlyOneActiveSelfPerUser(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&ProfileLinkPO{}))

	repo := NewRepository(db)
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	now := time.Now()

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, profileID := range []meta.ID{meta.FromUint64(2001), meta.FromUint64(2002)} {
		wg.Add(1)
		go func(profileID meta.ID) {
			defer wg.Done()
			<-start
			errs <- repo.Create(ctx, profilelink.NewSelfProfileLink(userID, profileID, now))
		}(profileID)
	}

	close(start)
	wg.Wait()
	close(errs)

	var successCount, duplicateCount int
	for err := range errs {
		switch {
		case err == nil:
			successCount++
		case perrors.IsCode(err, code.ErrIdentityProfileLinkExists):
			duplicateCount++
		default:
			require.NoError(t, err)
		}
	}

	require.Equal(t, 1, successCount)
	require.Equal(t, 1, duplicateCount)

	links, err := repo.FindByUserID(ctx, userID)
	require.NoError(t, err)
	var activeSelfCount int
	for _, link := range links {
		if link != nil && link.Type == profilelink.TypeSelf && link.IsActive() {
			activeSelfCount++
		}
	}
	assert.Equal(t, 1, activeSelfCount)
}

func TestRepository_UpdateRevokedSelfLinkReleasesActiveSelfGuard(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&ProfileLinkPO{}))

	repo := NewRepository(db)
	ctx := context.Background()
	userID := meta.FromUint64(1001)

	first := profilelink.NewSelfProfileLink(userID, meta.FromUint64(2001), time.Now())
	require.NoError(t, repo.Create(ctx, first))

	first.Revoke(time.Now())
	require.NoError(t, repo.Update(ctx, first))

	second := profilelink.NewSelfProfileLink(userID, meta.FromUint64(2002), time.Now())
	require.NoError(t, repo.Create(ctx, second))

	links, err := repo.FindByUserID(ctx, userID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, second.Profile, links[0].Profile)
}
