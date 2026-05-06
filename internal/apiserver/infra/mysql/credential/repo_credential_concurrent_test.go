package credential

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testutil "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/testutil"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	testhelpers "github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	m "github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// 并发创建相同的 credential（相同 account_id + idp + idp_identifier），期望只有 1 条记录被写入，
// 其余并发请求因唯一约束被 translator 映射为业务错误 code.ErrCredentialExists。
func TestCredentialRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	require.NoError(t, db.AutoMigrate(&PO{}))

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
			accountID := m.FromUint64(1) // 测试用 ID，必定有效
			cred := domain.NewPhoneOTPCredential(accountID, "+8613800000000")
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
	require.NoError(t, db.Model(&PO{}).
		Where("account_id = ? AND idp = ? AND idp_identifier = ?", 1, "phone", "+8613800000000").
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
