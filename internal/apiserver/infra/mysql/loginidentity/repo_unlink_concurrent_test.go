package loginidentity

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	testutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/testutil"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRepository_UnlinkOwnedUnlessLastActive_Concurrent(t *testing.T) {
	db := testutil.OpenDBForIntegrationTest(t, &PO{})
	repo := NewRepository(db)
	ctx := context.Background()
	userID := meta.FromUint64(uint64(time.Now().UnixNano()))

	identities := make([]*domain.LoginIdentity, 0, 2)
	for i := range 2 {
		key, err := domain.NewMockConsumerProviderKey(fmt.Sprintf("unlink-%d-%d", userID.Uint64(), i))
		require.NoError(t, err)
		identity, err := domain.NewBuilder(userID).
			FromProviderKey(key).
			Build()
		require.NoError(t, err)
		require.NoError(t, repo.Create(ctx, identity))
		identities = append(identities, identity)
	}
	t.Cleanup(func() {
		_ = db.Unscoped().Where("user_id = ?", userID).Delete(&PO{}).Error
	})

	outcomes := make(chan domain.UnlinkOutcome, len(identities))
	errs := make(chan error, len(identities))
	var wg sync.WaitGroup
	wg.Add(len(identities))
	for _, identity := range identities {
		go func(id meta.ID) {
			defer wg.Done()
			outcome, err := repo.UnlinkOwnedUnlessLastActive(ctx, userID, id)
			if err != nil {
				errs <- err
				return
			}
			outcomes <- outcome
		}(identity.ID)
	}
	wg.Wait()
	close(outcomes)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	counts := map[domain.UnlinkOutcome]int{}
	for outcome := range outcomes {
		counts[outcome]++
	}
	require.Equal(t, 1, counts[domain.UnlinkOutcomeUnlinked])
	require.Equal(t, 1, counts[domain.UnlinkOutcomeLastActive])

	stored, err := repo.ListByUserID(ctx, userID)
	require.NoError(t, err)
	active := 0
	for _, identity := range stored {
		if identity.Status == domain.StatusActive {
			active++
		}
	}
	require.Equal(t, 1, active)
}

func TestRepository_UnlinkOwnedUnlessLastActive_HidesForeignIdentity(t *testing.T) {
	db := testutil.OpenDBForIntegrationTest(t, &PO{})
	repo := NewRepository(db)
	ctx := context.Background()
	ownerID := meta.FromUint64(uint64(time.Now().UnixNano()))
	key, err := domain.NewMockConsumerProviderKey(fmt.Sprintf("foreign-%d", ownerID.Uint64()))
	require.NoError(t, err)
	identity, err := domain.NewBuilder(ownerID).
		FromProviderKey(key).
		Build()
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, identity))
	t.Cleanup(func() {
		_ = db.Unscoped().Where("id = ?", identity.ID).Delete(&PO{}).Error
	})

	outcome, err := repo.UnlinkOwnedUnlessLastActive(ctx, ownerID+1, identity.ID)
	require.NoError(t, err)
	require.Equal(t, domain.UnlinkOutcomeNotFound, outcome)
	stored, err := repo.GetByID(ctx, identity.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusActive, stored.Status)
}
