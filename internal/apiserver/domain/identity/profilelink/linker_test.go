package profilelink

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestProfileLinker_LinkRelationSuccess(t *testing.T) {
	profileLinkRepo := &stubProfileLinkRepo{profilesResults: make(map[uint64][]*ProfileLink)}
	manager := newLinkerWithClock(profileLinkRepo, func() time.Time {
		return time.Unix(10, 0)
	})

	profileLink, err := manager.LinkRelation(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.NoError(t, err)
	require.NotNil(t, profileLink)
	assert.Equal(t, meta.FromUint64(2), profileLink.User)
	assert.Equal(t, meta.FromUint64(1), profileLink.Profile)
	assert.Equal(t, TypeRelation, profileLink.Type)
	assert.Equal(t, RelParent, profileLink.Rel)
	assert.Equal(t, time.Unix(10, 0), profileLink.EstablishedAt)
}

func TestProfileLinker_LinkDispatchesSelfRelation(t *testing.T) {
	profileLinkRepo := &stubProfileLinkRepo{profilesResults: make(map[uint64][]*ProfileLink)}
	manager := newLinkerWithClock(profileLinkRepo, func() time.Time {
		return time.Unix(20, 0)
	})

	profileLink, err := manager.Link(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelSelf)

	require.NoError(t, err)
	require.NotNil(t, profileLink)
	assert.Equal(t, TypeSelf, profileLink.Type)
	assert.Equal(t, RelSelf, profileLink.Rel)
	assert.Equal(t, time.Unix(20, 0), profileLink.EstablishedAt)
}

func TestProfileLinker_LinkRejectsDuplicateActiveUserProfile(t *testing.T) {
	existing := &ProfileLink{User: meta.FromUint64(2), Profile: meta.FromUint64(1)}
	profileLinkRepo := &stubProfileLinkRepo{
		profilesResults: map[uint64][]*ProfileLink{
			1: {existing},
		},
	}
	manager := NewLinker(profileLinkRepo)

	profileLink, err := manager.Link(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "profile link already exists")
}

func TestProfileLinker_LinkAllowsMultipleRelationProfilesForUser(t *testing.T) {
	existingRelation := &ProfileLink{User: meta.FromUint64(10), Profile: meta.FromUint64(1), Type: TypeRelation, Rel: RelParent}
	profileLinkRepo := &stubProfileLinkRepo{
		userResults: map[uint64][]*ProfileLink{
			10: {existingRelation},
		},
		profilesResults: map[uint64][]*ProfileLink{
			2: {},
		},
	}
	manager := NewLinker(profileLinkRepo)

	profileLink, err := manager.LinkRelation(context.Background(), meta.FromUint64(10), meta.FromUint64(2), RelParent)

	require.NoError(t, err)
	require.NotNil(t, profileLink)
	assert.Equal(t, TypeRelation, profileLink.Type)
	assert.Equal(t, RelParent, profileLink.Rel)
}

func TestProfileLinker_LinkFindByProfileError(t *testing.T) {
	profileLinkRepo := &stubProfileLinkRepo{findErr: errors.New("db error")}
	manager := NewLinker(profileLinkRepo)

	profileLink, err := manager.Link(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find profile links failed")
}

func TestProfileLinker_RevokeSuccess(t *testing.T) {
	target := &ProfileLink{User: meta.FromUint64(2), Profile: meta.FromUint64(1)}
	profileLinkRepo := &stubProfileLinkRepo{
		profilesResults: map[uint64][]*ProfileLink{
			1: {target},
		},
	}
	manager := newLinkerWithClock(profileLinkRepo, func() time.Time {
		return time.Unix(30, 0)
	})

	removed, err := manager.Revoke(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.NoError(t, err)
	require.NotNil(t, removed)
	require.NotNil(t, removed.RevokedAt)
	assert.Equal(t, time.Unix(30, 0), *removed.RevokedAt)
}

func TestProfileLinker_RevokeNotFound(t *testing.T) {
	profileLinkRepo := &stubProfileLinkRepo{
		profilesResults: map[uint64][]*ProfileLink{
			1: {},
		},
	}
	manager := NewLinker(profileLinkRepo)

	removed, err := manager.Revoke(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.Error(t, err)
	assert.Nil(t, removed)
	assert.Contains(t, fmt.Sprintf("%-v", err), "active profile link not found")
}

func TestProfileLinker_RevokeFindError(t *testing.T) {
	profileLinkRepo := &stubProfileLinkRepo{findErr: errors.New("db error")}
	manager := NewLinker(profileLinkRepo)

	removed, err := manager.Revoke(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.Error(t, err)
	assert.Nil(t, removed)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find profile links failed")
}

func TestProfileLinker_LinkConcurrentDuplicateDetection(t *testing.T) {
	existing := &ProfileLink{User: meta.FromUint64(2), Profile: meta.FromUint64(1)}
	seq := &seqProfileLinkRepo{
		responses: [][]*ProfileLink{
			{},
			{existing},
		},
	}
	manager := NewLinker(seq)

	var wg sync.WaitGroup
	wg.Add(2)

	startCh := make(chan struct{})
	results := make([]struct {
		link *ProfileLink
		err  error
	}, 2)

	for i := 0; i < 2; i++ {
		idx := i
		go func() {
			defer wg.Done()
			<-startCh
			link, err := manager.Link(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)
			results[idx].link = link
			results[idx].err = err
		}()
	}

	close(startCh)
	wg.Wait()

	var success, duplicated int
	for _, result := range results {
		if result.err == nil && result.link != nil {
			success++
			continue
		}
		if result.err != nil && contains(fmt.Sprintf("%-v", result.err), "profile link already exists") {
			duplicated++
		}
	}

	assert.GreaterOrEqual(t, success, 1)
	assert.GreaterOrEqual(t, duplicated, 1)
}
