package credential

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/testutil"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	testhelpers "github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	m "github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// 并发创建相同的 credential（相同 login_identity_id + type），期望只有 1 条记录被写入，
// 其余并发请求因唯一约束被 translator 映射为业务错误 code.ErrCredentialExists。
func TestCredentialRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	require.NoError(t, db.AutoMigrate(&V2PO{}))

	repo := NewRepository(db)
	ctx := context.Background()

	const concurrency = 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errs := make(chan error, concurrency)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < concurrency; i++ {
		// compute delay in parent goroutine to avoid concurrent access to rng
		delay := rng.Intn(8)
		go func(d int) {
			defer wg.Done()
			// add tiny random delay to reduce SQLITE table-lock contention
			time.Sleep(time.Millisecond * time.Duration(d))
			loginIdentityID := m.FromUint64(1) // 测试用 ID，必定有效
			cred := domain.NewPasswordCredential(loginIdentityID, []byte("hash"), "argon2id")
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Create(ctx, cred) }); err != nil {
				errs <- err
				return
			}
			errs <- nil
		}(delay)
	}

	wg.Wait()
	close(errs)

	var success int
	var mappedCount int
	for e := range errs {
		if e == nil {
			success++
			continue
		}

		// unwrap chain to detect wrapped perrors-coded errors
		var ue error = e
		for ue != nil {
			if perrors.IsCode(ue, code.ErrCredentialExists) {
				mappedCount++
				break
			}
			ue = errors.Unwrap(ue)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrCredentialExists")

	var cnt int64
	require.NoError(t, db.Model(&V2PO{}).
		Where("login_identity_id = ? AND type = ?", 1, string(domain.CredPassword)).
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}

func TestCredentialRepository_RecordAuthenticationFailure_ConcurrentLockout(t *testing.T) {
	db := testutil.OpenDBForIntegrationTest(t, &V2PO{})
	repo := NewRepository(db)
	ctx := context.Background()
	cred := domain.NewPasswordCredential(m.FromUint64(uint64(time.Now().UnixNano())), []byte("hash"), "argon2id")
	require.NoError(t, repo.Create(ctx, cred))
	t.Cleanup(func() {
		_ = db.Unscoped().Where("id = ?", cred.ID.Uint64()).Delete(&V2PO{}).Error
	})

	const concurrency = 10
	now := time.Now().UTC().Truncate(time.Millisecond)
	policy := domain.LockoutPolicy{Enabled: true, Threshold: 5, LockDuration: 15 * time.Minute}
	var wg sync.WaitGroup
	wg.Add(concurrency)
	results := make(chan domain.AuthenticationState, concurrency)
	errs := make(chan error, concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			state, err := repo.ApplyAuthenticationTransition(ctx, domain.NewFailureTransition(cred.ID, now, policy))
			if err != nil {
				errs <- err
				return
			}
			results <- state
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	newLocks := 0
	for state := range results {
		if state.NewlyLocked {
			newLocks++
		}
	}
	require.Equal(t, 1, newLocks)
	found, err := repo.GetByID(ctx, cred.ID)
	require.NoError(t, err)
	require.Equal(t, concurrency, found.FailedAttempts)
	require.NotNil(t, found.LockedUntil)
	require.WithinDuration(t, now.Add(15*time.Minute), *found.LockedUntil, time.Second)
}
