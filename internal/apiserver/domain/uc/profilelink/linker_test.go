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

	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profile"
	userdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// package-local profileLink test helpers have been moved to
// ref_test_helpers.go to keep test file focused on behavior.

// profile and user repo stubs replaced by shared testhelpers stubs

func TestProfileLinker_CreateProfileLinkSuccess(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{Profile: &profiledomain.Profile{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	profileLinkRepo := &stubProfileLinkRepo{profilesResults: make(map[uint64][]*ProfileLink)}

	manager := NewLinker(profileLinkRepo, profileRepo, userRepo)

	profileLink, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.NoError(t, err)
	require.NotNil(t, profileLink)
	assert.Equal(t, meta.FromUint64(2), profileLink.User)
	assert.Equal(t, meta.FromUint64(1), profileLink.Profile)
	assert.Equal(t, RelParent, profileLink.Rel)
	assert.False(t, profileLink.EstablishedAt.IsZero())
}

func TestProfileLinker_Establish_Duplicate(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{Profile: &profiledomain.Profile{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	existing := &ProfileLink{User: meta.FromUint64(2), Profile: meta.FromUint64(1)}
	profileLinkRepo := &stubProfileLinkRepo{
		profilesResults: map[uint64][]*ProfileLink{
			1: {existing},
		},
	}

	manager := NewLinker(profileLinkRepo, profileRepo, userRepo)

	profileLink, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "profile link already exists")
}

func TestProfileLinker_Establish_ProfileNotFound(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{Profile: nil}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	profileLinkRepo := &stubProfileLinkRepo{}

	manager := NewLinker(profileLinkRepo, profileRepo, userRepo)

	profileLink, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "profile not found")
}

func TestProfileLinker_Establish_UserRepoError(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{Profile: &profiledomain.Profile{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.FindErr = errors.New("db error")
	profileLinkRepo := &stubProfileLinkRepo{}

	manager := NewLinker(profileLinkRepo, profileRepo, userRepo)

	profileLink, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find user failed")
}

func TestProfileLinker_Establish_FindByProfileError(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{Profile: &profiledomain.Profile{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	profileLinkRepo := &stubProfileLinkRepo{findErr: errors.New("db error")}

	manager := NewLinker(profileLinkRepo, profileRepo, userRepo)

	profileLink, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find profile links failed")
}

func TestProfileLinker_RemoveProfileLinkSuccess(t *testing.T) {
	target := &ProfileLink{User: meta.FromUint64(2), Profile: meta.FromUint64(1)}
	profileLinkRepo := &stubProfileLinkRepo{
		profilesResults: map[uint64][]*ProfileLink{
			1: {target},
		},
	}
	manager := NewLinker(profileLinkRepo, &testhelpers.ProfileRepoStub{}, testhelpers.NewUserRepoStub())

	removed, err := manager.Revoke(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.NoError(t, err)
	assert.NotNil(t, removed)
	assert.NotNil(t, removed.RevokedAt)
	assert.True(t, removed.RevokedAt.After(time.Time{}))
}

func TestProfileLinker_Revoke_NotFound(t *testing.T) {
	profileLinkRepo := &stubProfileLinkRepo{
		profilesResults: map[uint64][]*ProfileLink{
			1: {},
		},
	}
	manager := NewLinker(profileLinkRepo, &testhelpers.ProfileRepoStub{}, testhelpers.NewUserRepoStub())

	removed, err := manager.Revoke(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.Error(t, err)
	assert.Nil(t, removed)
	assert.Contains(t, fmt.Sprintf("%-v", err), "active profile link not found")
}

func TestProfileLinker_Establish_ProfileRepoError(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{FindErr: errors.New("db error")}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	profileLinkRepo := &stubProfileLinkRepo{}

	manager := NewLinker(profileLinkRepo, profileRepo, userRepo)

	profileLink, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find profile failed")
}

func TestProfileLinker_Establish_UserNotFound(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{Profile: &profiledomain.Profile{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	// ensure repo returns (nil, nil) for the id to simulate "user not found" without DB error
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = nil
	profileLinkRepo := &stubProfileLinkRepo{}

	manager := NewLinker(profileLinkRepo, profileRepo, userRepo)

	profileLink, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, profileLink)
	assert.Contains(t, fmt.Sprintf("%-v", err), "user not found")
}

func TestProfileLinker_Revoke_FindError(t *testing.T) {
	profileLinkRepo := &stubProfileLinkRepo{findErr: errors.New("db error")}
	manager := NewLinker(profileLinkRepo, &testhelpers.ProfileRepoStub{}, testhelpers.NewUserRepoStub())

	removed, err := manager.Revoke(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.Error(t, err)
	assert.Nil(t, removed)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find profile links failed")
}

// seqProfileLinkRepo and helper functions have been moved to ref_test_helpers.go
func TestProfileLinker_Establish_ConcurrentDuplicateDetection(t *testing.T) {
	profileRepo := &testhelpers.ProfileRepoStub{Profile: &profiledomain.Profile{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}

	existing := &ProfileLink{User: meta.FromUint64(2), Profile: meta.FromUint64(1)}
	seq := &seqProfileLinkRepo{
		responses: [][]*ProfileLink{
			{},         // first caller sees none
			{existing}, // second caller sees existing profileLink
		},
	}

	manager := NewLinker(seq, profileRepo, userRepo)

	var wg sync.WaitGroup
	wg.Add(2)

	startCh := make(chan struct{})
	results := make([]struct {
		g   *ProfileLink
		err error
	}, 2)

	for i := 0; i < 2; i++ {
		idx := i
		go func() {
			defer wg.Done()
			<-startCh
			g, err := manager.Establish(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)
			results[idx].g = g
			results[idx].err = err
		}()
	}

	// 同时开始，两者几乎同时调用 FindByProfileID
	close(startCh)
	wg.Wait()

	// 期望：一个成功，另一个因为已存在而失败
	var success, duplicated int
	for _, r := range results {
		if r.err == nil && r.g != nil {
			success++
		} else if r.err != nil {
			if contains(fmt.Sprintf("%-v", r.err), "profile link already exists") {
				duplicated++
			}
		}
	}

	// 要求至少有一个成功和至少一个重复错误
	assert.GreaterOrEqual(t, success, 1)
	assert.GreaterOrEqual(t, duplicated, 1)
}

// helper functions moved to ref_test_helpers.go
