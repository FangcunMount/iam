package profilelink_test

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/testutil"
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/identity/profilelink"
	store "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/profilelink"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRepository_RestoreChecksRevokedRowAndSelfGuard(t *testing.T) {
	db := testutil.OpenDBForIntegrationTest(t, &store.ProfileLinkPO{})
	repo := store.NewRepository(db)
	ctx := context.Background()
	original := restorableSelfLink(meta.FromUint64(18001001), meta.FromUint64(18002001), time.Now().Add(-time.Hour))
	require.NoError(t, repo.Create(ctx, original))
	t.Cleanup(func() { db.Where("user_id = ?", original.User.Uint64()).Delete(&store.ProfileLinkPO{}) })
	original.Revoke(time.Now().UTC().Truncate(time.Second))
	require.NoError(t, repo.Update(ctx, original))
	restored := restorableSelfLink(original.User, original.Profile, time.Now())
	restored.ID = original.ID
	require.Error(t, repo.Restore(ctx, restored, original.RevokedAt.Add(-time.Second)))
	wrong := restorableSelfLink(meta.FromUint64(18001002), original.Profile, time.Now())
	wrong.ID = original.ID
	require.Error(t, repo.Restore(ctx, wrong, *original.RevokedAt))
	second := restorableSelfLink(original.User, meta.FromUint64(18002002), time.Now())
	require.NoError(t, repo.Create(ctx, second))
	require.Error(t, repo.Restore(ctx, restored, *original.RevokedAt))
	second.Revoke(time.Now().UTC().Truncate(time.Second))
	require.NoError(t, repo.Update(ctx, second))
	require.NoError(t, repo.Restore(ctx, restored, *original.RevokedAt))
	require.Error(t, repo.Restore(ctx, restored, *original.RevokedAt))
	actual, err := repo.FindByID(ctx, original.ID)
	require.NoError(t, err)
	require.True(t, actual.IsActive())
	require.Equal(t, domain.RelSelf, actual.Rel)
	var row store.ProfileLinkPO
	require.NoError(t, db.First(&row, "id = ?", original.ID.Uint64()).Error)
	require.Equal(t, uint32(2), row.Version)
}

func restorableSelfLink(user, profile meta.ID, at time.Time) *domain.ProfileLink {
	return &domain.ProfileLink{User: user, Profile: profile, Type: domain.TypeSelf, Rel: domain.RelSelf, EstablishedAt: at}
}
