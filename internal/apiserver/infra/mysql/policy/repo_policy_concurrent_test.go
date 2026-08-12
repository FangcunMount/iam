package policy

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/testutil"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	testhelpers "github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

// 并发创建相同的 policy version（相同 tenant + version），期望只有 1 条记录被写入，
// 其余并发请求因唯一约束被 translator 映射为业务错误 code.ErrPolicyVersionAlreadyExists。
func TestPolicyVersionRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	require.NoError(t, db.AutoMigrate(&PolicyVersionPO{}))

	repoIface := NewPolicyVersionRepository(db)
	pr, ok := repoIface.(*PolicyVersionRepository)
	require.True(t, ok)
	repo := pr
	ctx := context.Background()

	const concurrency = 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errs := make(chan error, concurrency)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tenant := "tenant-concurrent"
	version := int64(1)
	for i := 0; i < concurrency; i++ {
		// compute delay in parent goroutine to avoid concurrent access to rng
		delay := rng.Intn(8)
		go func(d int) {
			defer wg.Done()
			time.Sleep(time.Millisecond * time.Duration(d))
			pv := domain.NewPolicyVersion(tenant, version)
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Create(ctx, &pv) }); err != nil {
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

		var ue error = e
		for ue != nil {
			if perrors.IsCode(ue, code.ErrPolicyVersionAlreadyExists) {
				mappedCount++
				break
			}
			ue = errors.Unwrap(ue)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrPolicyVersionAlreadyExists")

	var cnt int64
	require.NoError(t, db.Model(&PolicyVersionPO{}).
		Where("tenant_id = ? AND policy_version = ?", tenant, version).
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}

func TestPolicyVersionRepository_Increment_ConcurrentCallsCreateSequentialVersions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	require.NoError(t, db.AutoMigrate(&PolicyVersionPO{}))

	repoIface := NewPolicyVersionRepository(db)
	repo, ok := repoIface.(*PolicyVersionRepository)
	require.True(t, ok)
	ctx := context.Background()

	seed := domain.NewPolicyVersion("tenant-increment", 1)
	require.NoError(t, repo.Create(ctx, &seed))

	const concurrency = 8
	var wg sync.WaitGroup
	wg.Add(concurrency)
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, err := repo.Increment(ctx, "tenant-increment", "operator", "concurrent increment")
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	current, err := repo.GetCurrent(ctx, "tenant-increment")
	require.NoError(t, err)
	require.NotNil(t, current)
	require.Equal(t, int64(1+concurrency), current.Version)

	var count int64
	require.NoError(t, db.Model(&PolicyVersionPO{}).Where("tenant_id = ?", "tenant-increment").Count(&count).Error)
	require.Equal(t, int64(1+concurrency), count)

	var versions []int64
	require.NoError(t, db.Model(&PolicyVersionPO{}).
		Where("tenant_id = ?", "tenant-increment").
		Order("policy_version ASC").
		Pluck("policy_version", &versions).Error)
	require.Equal(t, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9}, versions)
}
