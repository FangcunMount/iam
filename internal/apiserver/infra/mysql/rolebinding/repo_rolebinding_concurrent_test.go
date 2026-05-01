package rolebinding

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	testhelpers "github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

// 并发创建相同的 binding（相同 subject_type+subject_id+role_id+tenant_id），
// 在测试环境为表添加唯一索引以触发重复错误，期望只有 1 条记录写入，
// 其余被翻译为 code.ErrAssignmentAlreadyExists。
func TestBindingRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&BindingPO{}))

	// 为测试环境显式创建唯一索引，避免在 PO tag 中改动生产 schema
	// 复合唯一键: subject_type, subject_id, role_id, tenant_id
	_ = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uk_binding ON authz_assignments(subject_type, subject_id, role_id, tenant_id)")

	repo := NewBindingRepository(db)
	ctx := context.Background()

	const concurrency = 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errs := make(chan error, concurrency)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < concurrency; i++ {
		delay := rng.Intn(8)
		go func(d int) {
			defer wg.Done()
			time.Sleep(time.Millisecond * time.Duration(d))
			a := domain.NewBinding(domain.SubjectTypeUser, "user-123", 42, "tenant-1")
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Create(ctx, &a) }); err != nil {
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
			if perrors.IsCode(ue, code.ErrAssignmentAlreadyExists) {
				mappedCount++
				break
			}
			ue = errors.Unwrap(ue)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrAssignmentAlreadyExists")

	var cnt int64
	require.NoError(t, db.Model(&BindingPO{}).
		Where("subject_type = ? AND subject_id = ? AND role_id = ? AND tenant_id = ?", "user", "user-123", 42, "tenant-1").
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
