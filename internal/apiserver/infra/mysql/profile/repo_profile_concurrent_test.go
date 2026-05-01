package profile

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profile"
	testhelpers "github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	m "github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// 并发创建相同的档案（相同身份证号），期望只有 1 条记录被写入，
// 其余并发请求因唯一约束被 translator 映射为业务错误 code.ErrIdentityProfileExists。
func TestProfileRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&ProfilePO{}))

	repo := NewRepository(db)
	ctx := context.Background()

	const concurrency = 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errs := make(chan error, concurrency)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	idNumber := "110101199003070011" // 有效的测试身份证号
	for i := 0; i < concurrency; i++ {
		delay := rng.Intn(8)
		go func(d int) {
			defer wg.Done()
			// tiny jitter to reduce lock storms on SQLite
			time.Sleep(time.Millisecond * time.Duration(d))
			idCard, err := m.NewIDCard("Alice", idNumber)
			if err != nil {
				errs <- err
				return
			}
			c, err := domain.NewProfile("Alice", domain.WithIDCard(idCard))
			if err != nil {
				errs <- err
				return
			}
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Create(ctx, c) }); err != nil {
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
			if perrors.IsCode(ue, code.ErrIdentityProfileExists) {
				mappedCount++
				break
			}
			ue = errors.Unwrap(ue)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrIdentityProfileExists")

	var cnt int64
	require.NoError(t, db.Model(&ProfilePO{}).
		Where("id_card = ?", idNumber).
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
