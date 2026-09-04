package scheduler

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/robfig/cron/v3"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signingkey"
)

// KeyRotationCronScheduler 基于 Cron 表达式的密钥轮换调度器
// 相比 Ticker 方式，Cron 更适合长周期任务，资源消耗更少
type KeyRotationCronScheduler struct {
	lifecycleApp *signingkey.KeyLifecycleAppService
	logger       log.Logger

	cronSpec string     // Cron 表达式，如 "0 2 * * *" 表示每天凌晨2点
	cron     *cron.Cron // Cron 调度器
	entryID  cron.EntryID

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.RWMutex
	running bool
}

// NewKeyRotationCronScheduler 创建基于 Cron 的密钥轮换调度器
//
// cronSpec 示例：
//   - "0 2 * * *"        每天凌晨2点执行
//   - "0 2 */3 * *"      每3天凌晨2点执行
//   - "0 2 1 * *"        每月1号凌晨2点执行
//   - "@daily"           每天午夜执行
//   - "@weekly"          每周日午夜执行
//   - "@monthly"         每月1号午夜执行
//   - "@every 24h"       每24小时执行（推荐用于密钥轮换）
//   - "@every 720h"      每30天执行
func NewKeyRotationCronScheduler(
	lifecycleApp *signingkey.KeyLifecycleAppService,
	cronSpec string,
	logger log.Logger,
) *KeyRotationCronScheduler {
	if cronSpec == "" {
		// 默认每天凌晨2点检查一次
		// 这样即使密钥轮换周期是30天，也只需要在凌晨检查
		cronSpec = "0 2 * * *"
	}

	return &KeyRotationCronScheduler{
		lifecycleApp: lifecycleApp,
		logger:       logger,
		cronSpec:     cronSpec,
	}
}

// Start 启动调度器
func (s *KeyRotationCronScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.logger.Warn("Key rotation cron scheduler is already running")
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// 创建 Cron 调度器
	// 注意：这里简化实现，不使用 cronlog 适配器，因为 Logger 接口没有暴露底层 zap.Logger
	s.cron = cron.New()

	// 添加密钥轮换任务
	entryID, err := s.cron.AddFunc(s.cronSpec, func() {
		if err := s.checkAndRotate(s.ctx); err != nil {
			s.logger.Errorw("Scheduled key rotation check failed", "error", err)
		}
	})
	if err != nil {
		s.logger.Errorw("Failed to add cron job", "error", err, "cronSpec", s.cronSpec)
		return err
	}

	s.entryID = entryID
	s.cron.Start()
	s.running = true

	s.logger.Infow("Key rotation cron scheduler started",
		"cronSpec", s.cronSpec,
		"nextRun", s.cron.Entry(entryID).Next,
	)

	return nil
}

// Stop 停止调度器
func (s *KeyRotationCronScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		s.logger.Warn("Key rotation cron scheduler is not running")
		return nil
	}

	s.logger.Info("Stopping key rotation cron scheduler...")

	// 停止 Cron 调度器（会等待正在执行的任务完成）
	ctx := s.cron.Stop()
	<-ctx.Done()

	s.cancel()
	s.running = false

	s.logger.Info("Key rotation cron scheduler stopped")
	return nil
}

// IsRunning 返回调度器是否正在运行
func (s *KeyRotationCronScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// TriggerNow 立即触发一次密钥轮换检查
func (s *KeyRotationCronScheduler) TriggerNow(ctx context.Context) error {
	s.logger.Info("Manually triggering key rotation check")
	return s.checkAndRotate(ctx)
}

// GetNextRunTime 获取下次执行时间
func (s *KeyRotationCronScheduler) GetNextRunTime() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running || s.cron == nil {
		return "not scheduled"
	}

	entry := s.cron.Entry(s.entryID)
	if entry.ID == 0 {
		return "unknown"
	}

	return entry.Next.Format("2006-01-02 15:04:05")
}

// checkAndRotate 检查并执行密钥轮换
func (s *KeyRotationCronScheduler) checkAndRotate(ctx context.Context) error {
	resp, err := s.lifecycleApp.RotateIfDue(ctx)
	if err != nil {
		schedulerExecutions.WithLabelValues("failed").Inc()
		s.logger.Errorw("Automatic key rotation failed", "result", "failed")
		return err
	}
	if resp == nil || !resp.Rotated {
		schedulerExecutions.WithLabelValues("noop").Inc()
		s.logger.Debugw("Key rotation not needed", "result", "noop")
		return nil
	}

	schedulerExecutions.WithLabelValues("success").Inc()
	s.logger.Infow("Automatic key rotation completed successfully",
		"kid", resp.NewKey.Kid,
		"algorithm", resp.NewKey.Algorithm,
		"status", resp.NewKey.Status,
	)

	return nil
}

var schedulerExecutions = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "jwks",
	Name:      "scheduler_executions_total",
	Help:      "JWKS scheduler executions by result.",
}, []string{"result"})
