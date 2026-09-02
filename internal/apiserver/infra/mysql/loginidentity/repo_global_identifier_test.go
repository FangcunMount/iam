package loginidentity

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/testutil"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryCreateStoresOneCanonicalGlobalIdentifier(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PO{}))
	repo := NewRepository(db)
	ctx := context.Background()

	first := newGlobalIdentity(t, 100, "wx-app-a", "openid-a", "union-1")
	require.NoError(t, repo.Create(ctx, first))
	require.Equal(t, "union-1", first.GlobalIdentifier)

	sameOwnerOtherRealm := newGlobalIdentity(t, 100, "wx-app-b", "openid-b", "union-1")
	require.NoError(t, repo.Create(ctx, sameOwnerOtherRealm))
	require.Empty(t, sameOwnerOtherRealm.GlobalIdentifier)

	otherOwner := newGlobalIdentity(t, 200, "wx-app-c", "openid-c", "union-1")
	err = repo.Create(ctx, otherOwner)
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrGlobalIdentifierExists), "error = %v", err)

	var identityCount, canonicalCount int64
	require.NoError(t, db.Model(&PO{}).Count(&identityCount).Error)
	require.NoError(t, db.Model(&PO{}).
		Where("provider = ? AND global_identifier = ?", domain.ProviderWechatMinip, "union-1").
		Count(&canonicalCount).Error)
	require.Equal(t, int64(2), identityCount)
	require.Equal(t, int64(1), canonicalCount)
}

func TestRepositoryCreateMovesInactiveCanonicalGlobalIdentifier(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PO{}))
	repo := NewRepository(db)
	ctx := context.Background()

	first := newGlobalIdentity(t, 100, "wx-app-a", "openid-a", "union-1")
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, db.Model(&PO{}).Where("id = ?", first.ID).Update("status", string(domain.StatusDeleted)).Error)

	replacement := newGlobalIdentity(t, 100, "wx-app-b", "openid-b", "union-1")
	require.NoError(t, repo.Create(ctx, replacement))
	require.Equal(t, "union-1", replacement.GlobalIdentifier)

	var oldPO, replacementPO PO
	require.NoError(t, db.First(&oldPO, "id = ?", first.ID).Error)
	require.NoError(t, db.First(&replacementPO, "id = ?", replacement.ID).Error)
	require.Nil(t, oldPO.GlobalIdentifier)
	require.NotNil(t, replacementPO.GlobalIdentifier)
	require.Equal(t, "union-1", *replacementPO.GlobalIdentifier)
}

func TestRepositoryUnlinkTransfersCanonicalGlobalIdentifier(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PO{}))
	repo := NewRepository(db)
	ctx := context.Background()

	first := newGlobalIdentity(t, 100, "wx-app-a", "openid-a", "union-1")
	second := newGlobalIdentity(t, 100, "wx-app-b", "openid-b", "union-1")
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	outcome, err := repo.UnlinkOwnedUnlessLastActive(ctx, meta.FromUint64(100), first.ID)
	require.NoError(t, err)
	require.Equal(t, domain.UnlinkOutcomeUnlinked, outcome)

	var firstPO, secondPO PO
	require.NoError(t, db.First(&firstPO, "id = ?", first.ID).Error)
	require.NoError(t, db.First(&secondPO, "id = ?", second.ID).Error)
	require.Equal(t, string(domain.StatusDeleted), firstPO.Status)
	require.Nil(t, firstPO.GlobalIdentifier)
	require.NotNil(t, secondPO.GlobalIdentifier)
	require.Equal(t, "union-1", *secondPO.GlobalIdentifier)
}

func TestRepositoryCreateSerializesConcurrentGlobalIdentifierClaims(t *testing.T) {
	db := testutil.OpenDBForIntegrationTest(t, &PO{})
	repo := NewRepository(db)
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	globalIdentifier := fmt.Sprintf("union-concurrent-%d", suffix)
	identities := []*domain.LoginIdentity{
		newGlobalIdentity(t, uint64(suffix), "wx-app-concurrent-a", fmt.Sprintf("openid-a-%d", suffix), globalIdentifier),
		newGlobalIdentity(t, uint64(suffix)+1, "wx-app-concurrent-b", fmt.Sprintf("openid-b-%d", suffix), globalIdentifier),
	}
	t.Cleanup(func() {
		_ = db.Unscoped().Where("global_identifier = ? OR identifier IN ?", globalIdentifier, []string{identities[0].Identifier, identities[1].Identifier}).Delete(&PO{}).Error
	})

	errs := make(chan error, len(identities))
	var wg sync.WaitGroup
	wg.Add(len(identities))
	for _, identity := range identities {
		go func(candidate *domain.LoginIdentity) {
			defer wg.Done()
			errs <- repo.Create(ctx, candidate)
		}(identity)
	}
	wg.Wait()
	close(errs)

	successes, conflicts := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case perrors.IsCode(err, code.ErrGlobalIdentifierExists):
			conflicts++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func newGlobalIdentity(t *testing.T, userID uint64, realm, identifier, globalIdentifier string) *domain.LoginIdentity {
	t.Helper()
	key, err := domain.NewWechatMinipProviderKey(realm, identifier, globalIdentifier)
	require.NoError(t, err)
	identity, err := domain.NewBuilder(meta.FromUint64(userID)).FromProviderKey(key).Build()
	require.NoError(t, err)
	return identity
}
