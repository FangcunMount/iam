package assignment

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	testhelpers "github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// 并发创建相同的 assignment（相同 subject_type+subject_id+role_id+tenant_id），
// 直接使用 AssignmentPO 映射的规范 schema（对应 migration 000025）验证唯一保护，
// 期望只有 1 条记录写入，其余被翻译为 code.ErrAssignmentAlreadyExists。
func TestRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&AssignmentPO{}))
	require.True(t, db.Migrator().HasIndex(&AssignmentPO{}, "uk_authz_assignments_active"))

	repo := NewRepository(db)
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
			a, err := domain.NewAssignment(
				domain.SubjectTypeUser,
				meta.FromUint64(123),
				meta.FromUint64(42),
				"tenant-1",
				domain.WithGrantedBy("admin"),
			)
			if err != nil {
				errs <- err
				return
			}
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
	var unexpected []error
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
		if ue == nil {
			unexpected = append(unexpected, e)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.Empty(t, unexpected, "all failed creates must be unique-conflict business errors")
	require.Equal(t, concurrency-1, mappedCount, "every duplicate create should map to ErrAssignmentAlreadyExists")

	var cnt int64
	require.NoError(t, db.Model(&AssignmentPO{}).
		Where("subject_type = ? AND subject_id = ? AND role_id = ? AND tenant_id = ?", "user", "123", 42, "tenant-1").
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}

func TestRepository_Create_AllowsRegrantAfterHistoricalDeletion(t *testing.T) {
	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&AssignmentPO{}))
	repo := NewRepository(db)
	ctx := context.Background()

	first, err := domain.NewAssignment(
		domain.SubjectTypeUser,
		meta.FromUint64(123),
		meta.FromUint64(42),
		"tenant-1",
		domain.WithGrantedBy("admin"),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, &first))
	require.NoError(t, db.Model(&AssignmentPO{}).
		Where("id = ?", first.ID.Uint64()).
		Update("deleted_at", time.Now().UTC()).Error)

	second, err := domain.NewAssignment(
		domain.SubjectTypeUser,
		meta.FromUint64(123),
		meta.FromUint64(42),
		"tenant-1",
		domain.WithGrantedBy("admin"),
	)
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, &second))

	var activeCount int64
	require.NoError(t, db.Model(&AssignmentPO{}).
		Where("subject_type = ? AND subject_id = ? AND role_id = ? AND tenant_id = ? AND deleted_at IS NULL", "user", "123", 42, "tenant-1").
		Count(&activeCount).Error)
	require.Equal(t, int64(1), activeCount)
}
