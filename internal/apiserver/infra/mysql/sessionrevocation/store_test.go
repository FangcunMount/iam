package sessionrevocation

import (
	"context"
	"errors"
	"testing"
	"time"

	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	userpo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStoreStageParticipatesInCallerTransaction(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	userID := meta.FromUint64(100)
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, name, nickname, phone, email, status, version) VALUES (?, ?, '', '', '', 1, 7)",
		userID.Uint64(), "test",
	).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, NewStore(tx).Stage(context.Background(), userID, "revoke_all", "user_blocked"))
		return errors.New("rollback")
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&Task{}).Count(&count).Error)
	require.Zero(t, count)

	require.NoError(t, NewStore(db).Stage(context.Background(), userID, "revoke_all", "user_blocked"))
	var task Task
	require.NoError(t, db.First(&task).Error)
	require.Equal(t, uint32(7), task.UserVersion)
}

func TestWorkerRetriesThenCompletes(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	userID := meta.FromUint64(101)
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, name, nickname, phone, email, status, version) VALUES (?, ?, '', '', '', 1, 2)",
		userID.Uint64(), "test",
	).Error)
	store := NewStore(db)
	require.NoError(t, store.Stage(context.Background(), userID, "revoke_all", "user_deactivated"))

	revoker := &revokerStub{failures: 1}
	worker := NewWorker(store, revoker, WorkerConfig{
		PollInterval:         time.Millisecond,
		BatchSize:            10,
		RetryBaseDelay:       time.Millisecond,
		RetryMaxDelay:        5 * time.Millisecond,
		StaleProcessingAfter: time.Second,
	})
	worker.runBatch(context.Background())
	var task Task
	require.NoError(t, db.First(&task).Error)
	require.Equal(t, StatusFailed, task.Status)

	time.Sleep(5 * time.Millisecond)
	worker.runBatch(context.Background())
	var completed Task
	require.NoError(t, db.First(&completed, task.TaskID).Error)
	require.Equal(t, StatusCompleted, completed.Status)
	require.Equal(t, 2, revoker.calls)
	require.Equal(t, "iam:identity-status", revoker.revokedBy)
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&userpo.UserPO{}, &Task{}))
	return db
}

type revokerStub struct {
	failures  int
	calls     int
	revokedBy string
}

func (r *revokerStub) Revoke(context.Context, string, string, string) error { return nil }
func (r *revokerStub) RevokeByLoginIdentity(context.Context, meta.ID, string, string) error {
	return nil
}
func (r *revokerStub) RevokeByUser(_ context.Context, _ meta.ID, _ string, revokedBy string) error {
	r.calls++
	r.revokedBy = revokedBy
	if r.failures > 0 {
		r.failures--
		return errors.New("redis unavailable")
	}
	return nil
}

var _ sessiondomain.Revoker = (*revokerStub)(nil)
