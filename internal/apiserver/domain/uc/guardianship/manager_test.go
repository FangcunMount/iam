package guardianship

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	childdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	userdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// package-local guardianship test helpers have been moved to
// guardianship_test_helpers.go to keep test file focused on behavior.

// child and user repo stubs replaced by shared testhelpers stubs

func TestGuardianshipManager_AddGuardianSuccess(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{Child: &childdomain.Child{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	guardRepo := &stubGuardianshipRepo{childrenResults: make(map[uint64][]*Guardianship)}

	manager := NewManagerService(guardRepo, childRepo, userRepo)

	guardian, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.NoError(t, err)
	require.NotNil(t, guardian)
	assert.Equal(t, meta.FromUint64(2), guardian.User)
	assert.Equal(t, meta.FromUint64(1), guardian.Child)
	assert.Equal(t, RelParent, guardian.Rel)
	assert.False(t, guardian.EstablishedAt.IsZero())
}

func TestGuardianshipManager_AddGuardian_Duplicate(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{Child: &childdomain.Child{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	existing := &Guardianship{User: meta.FromUint64(2), Child: meta.FromUint64(1)}
	guardRepo := &stubGuardianshipRepo{
		childrenResults: map[uint64][]*Guardianship{
			1: {existing},
		},
	}

	manager := NewManagerService(guardRepo, childRepo, userRepo)

	guardian, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, guardian)
	assert.Contains(t, fmt.Sprintf("%-v", err), "guardian already exists")
}

func TestGuardianshipManager_AddGuardian_ChildNotFound(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{Child: nil}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	guardRepo := &stubGuardianshipRepo{}

	manager := NewManagerService(guardRepo, childRepo, userRepo)

	guardian, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, guardian)
	assert.Contains(t, fmt.Sprintf("%-v", err), "child not found")
}

func TestGuardianshipManager_AddGuardian_UserRepoError(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{Child: &childdomain.Child{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.FindErr = errors.New("db error")
	guardRepo := &stubGuardianshipRepo{}

	manager := NewManagerService(guardRepo, childRepo, userRepo)

	guardian, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, guardian)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find user failed")
}

func TestGuardianshipManager_AddGuardian_FindByChildError(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{Child: &childdomain.Child{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	guardRepo := &stubGuardianshipRepo{findErr: errors.New("db error")}

	manager := NewManagerService(guardRepo, childRepo, userRepo)

	guardian, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, guardian)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find guardians failed")
}

func TestGuardianshipManager_RemoveGuardianSuccess(t *testing.T) {
	target := &Guardianship{User: meta.FromUint64(2), Child: meta.FromUint64(1)}
	guardRepo := &stubGuardianshipRepo{
		childrenResults: map[uint64][]*Guardianship{
			1: {target},
		},
	}
	manager := NewManagerService(guardRepo, &testhelpers.ChildRepoStub{}, testhelpers.NewUserRepoStub())

	removed, err := manager.RemoveGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.NoError(t, err)
	assert.NotNil(t, removed)
	assert.NotNil(t, removed.RevokedAt)
	assert.True(t, removed.RevokedAt.After(time.Time{}))
}

func TestGuardianshipManager_RemoveGuardian_NotFound(t *testing.T) {
	guardRepo := &stubGuardianshipRepo{
		childrenResults: map[uint64][]*Guardianship{
			1: {},
		},
	}
	manager := NewManagerService(guardRepo, &testhelpers.ChildRepoStub{}, testhelpers.NewUserRepoStub())

	removed, err := manager.RemoveGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.Error(t, err)
	assert.Nil(t, removed)
	assert.Contains(t, fmt.Sprintf("%-v", err), "active guardian not found")
}

func TestGuardianshipManager_AddGuardian_ChildRepoError(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{FindErr: errors.New("db error")}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}
	guardRepo := &stubGuardianshipRepo{}

	manager := NewManagerService(guardRepo, childRepo, userRepo)

	guardian, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, guardian)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find child failed")
}

func TestGuardianshipManager_AddGuardian_UserNotFound(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{Child: &childdomain.Child{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	// ensure repo returns (nil, nil) for the id to simulate "user not found" without DB error
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = nil
	guardRepo := &stubGuardianshipRepo{}

	manager := NewManagerService(guardRepo, childRepo, userRepo)

	guardian, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)

	require.Error(t, err)
	assert.Nil(t, guardian)
	assert.Contains(t, fmt.Sprintf("%-v", err), "user not found")
}

func TestGuardianshipManager_RemoveGuardian_FindError(t *testing.T) {
	guardRepo := &stubGuardianshipRepo{findErr: errors.New("db error")}
	manager := NewManagerService(guardRepo, &testhelpers.ChildRepoStub{}, testhelpers.NewUserRepoStub())

	removed, err := manager.RemoveGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1))

	require.Error(t, err)
	assert.Nil(t, removed)
	assert.Contains(t, fmt.Sprintf("%-v", err), "find guardians failed")
}

// seqGuardRepo and helper functions have been moved to guardianship_test_helpers.go
func TestGuardianshipManager_AddGuardian_ConcurrentDuplicateDetection(t *testing.T) {
	childRepo := &testhelpers.ChildRepoStub{Child: &childdomain.Child{ID: meta.FromUint64(1)}}
	userRepo := testhelpers.NewUserRepoStub()
	userRepo.UsersByID[meta.FromUint64(2).Uint64()] = &userdomain.User{ID: meta.FromUint64(2)}

	existing := &Guardianship{User: meta.FromUint64(2), Child: meta.FromUint64(1)}
	seq := &seqGuardRepo{
		responses: [][]*Guardianship{
			{},         // first caller sees none
			{existing}, // second caller sees existing guardian
		},
	}

	manager := NewManagerService(seq, childRepo, userRepo)

	var wg sync.WaitGroup
	wg.Add(2)

	startCh := make(chan struct{})
	results := make([]struct {
		g   *Guardianship
		err error
	}, 2)

	for i := 0; i < 2; i++ {
		idx := i
		go func() {
			defer wg.Done()
			<-startCh
			g, err := manager.AddGuardian(context.Background(), meta.FromUint64(2), meta.FromUint64(1), RelParent)
			results[idx].g = g
			results[idx].err = err
		}()
	}

	// 同时开始，两者几乎同时调用 FindByChildID
	close(startCh)
	wg.Wait()

	// 期望：一个成功，另一个因为已存在而失败
	var success, duplicated int
	for _, r := range results {
		if r.err == nil && r.g != nil {
			success++
		} else if r.err != nil {
			if contains(fmt.Sprintf("%-v", r.err), "guardian already exists") {
				duplicated++
			}
		}
	}

	// 要求至少有一个成功和至少一个重复错误
	assert.GreaterOrEqual(t, success, 1)
	assert.GreaterOrEqual(t, duplicated, 1)
}

// helper functions moved to guardianship_test_helpers.go
