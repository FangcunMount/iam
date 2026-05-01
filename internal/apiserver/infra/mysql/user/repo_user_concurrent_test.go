package user

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
	testhelpers "github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	m "github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// 并发创建相同身份证的用户，期望只有 1 条记录被写入，
// 其余并发请求因唯一约束被 translator 映射为业务错误 code.ErrUserAlreadyExists。
func TestUserRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&UserPO{}))

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
			time.Sleep(time.Millisecond * time.Duration(d))
			phone, err := m.NewPhone("+8613900000000")
			if err != nil {
				errs <- err
				return
			}
			idCard, err := m.NewIDCard("Bob", idNumber)
			if err != nil {
				errs <- err
				return
			}
			u, err := domain.NewUser("Bob", phone, domain.WithIDCard(idCard))
			if err != nil {
				errs <- err
				return
			}
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Create(ctx, u) }); err != nil {
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
			if perrors.IsCode(ue, code.ErrUserAlreadyExists) {
				mappedCount++
				break
			}
			ue = errors.Unwrap(ue)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrUserAlreadyExists")

	var cnt int64
	require.NoError(t, db.Model(&UserPO{}).
		Where("id_card = ?", idNumber).
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
