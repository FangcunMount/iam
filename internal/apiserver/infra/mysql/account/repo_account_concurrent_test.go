package account

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testutil "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/testutil"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	testhelpers "github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// 并发创建相同的 account（相同 type+app_id+external_id），期望只有 1 条记录被写入，
// 其余并发请求因唯一约束被 translator 映射为业务错误 code.ErrAccountExists。
func TestAccountRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	// ensure table for this repo
	require.NoError(t, db.AutoMigrate(&AccountPO{}))

	repo := NewAccountRepository(db)
	ctx := context.Background()

	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errs := make(chan error, concurrency)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < concurrency; i++ {
		delay := rng.Intn(8)
		go func(d int) {
			defer wg.Done()
			// add tiny random delay to reduce SQLITE table-lock contention
			time.Sleep(time.Millisecond * time.Duration(d))
			userID := meta.FromUint64(1) // 测试用 ID，必定有效
			acc := domain.NewAccount(userID, domain.TypeWcMinip, domain.ExternalID("ext-dup"), domain.WithAppID(domain.AppId("app-1")))
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Create(ctx, acc) }); err != nil {
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
		// debug: log error type/message to diagnose translation behavior
		if e != nil {
			t.Logf("err: %T: %v", e, e)
		}
		if e == nil {
			success++
			continue
		}

		// unwrap chain to detect wrapped perrors-coded errors
		var ue error = e
		for ue != nil {
			if perrors.IsCode(ue, code.ErrAccountExists) {
				mappedCount++
				break
			}
			ue = errors.Unwrap(ue)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrAccountExists")

	var cnt int64
	require.NoError(t, db.Model(&AccountPO{}).
		Where("type = ? AND app_id = ? AND external_id = ?", string(domain.TypeWcMinip), "app-1", "ext-dup").
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
