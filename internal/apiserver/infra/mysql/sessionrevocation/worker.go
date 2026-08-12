package sessionrevocation

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
)

type WorkerConfig struct {
	PollInterval         time.Duration
	BatchSize            int
	RetryBaseDelay       time.Duration
	RetryMaxDelay        time.Duration
	StaleProcessingAfter time.Duration
}

type Worker struct {
	store   *Store
	revoker sessiondomain.Revoker
	config  WorkerConfig
}

func NewWorker(store *Store, revoker sessiondomain.Revoker, config WorkerConfig) *Worker {
	return &Worker{store: store, revoker: revoker, config: config}
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.store == nil || w.revoker == nil {
		return
	}
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	for {
		w.runBatch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) runBatch(ctx context.Context) {
	defer w.recordState(ctx)
	tasks, err := w.store.Claim(ctx, w.config.BatchSize, time.Now().UTC().Add(-w.config.StaleProcessingAfter))
	if err != nil {
		log.Warnw("session revocation claim failed", "operation", "claim", "result", "failed")
		return
	}
	for _, task := range tasks {
		if err := w.revoker.RevokeByUser(ctx, task.UserID, task.Reason, "iam:identity-status"); err != nil {
			next := time.Now().UTC().Add(w.retryDelay(task.AttemptCount))
			if storeErr := w.store.Fail(ctx, task.TaskID, next); storeErr != nil {
				log.Warnw("session revocation task update failed", "operation", "retry", "result", "failed")
			}
			continue
		}
		if err := w.store.Complete(ctx, task.TaskID); err != nil {
			log.Warnw("session revocation completion failed", "operation", "complete", "result", "failed")
		}
	}
}

func (w *Worker) recordState(ctx context.Context) {
	counts, err := w.store.StatusCounts(ctx)
	if err != nil {
		return
	}
	age, err := w.store.OldestUnfinishedAge(ctx, time.Now().UTC())
	if err != nil {
		return
	}
	recordTaskState(counts, age.Seconds())
}

func (w *Worker) retryDelay(attempt uint32) time.Duration {
	delay := w.config.RetryBaseDelay
	for i := uint32(1); i < attempt && delay < w.config.RetryMaxDelay; i++ {
		if delay > w.config.RetryMaxDelay/2 {
			return w.config.RetryMaxDelay
		}
		delay *= 2
	}
	if delay > w.config.RetryMaxDelay {
		return w.config.RetryMaxDelay
	}
	return delay
}
