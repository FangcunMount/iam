package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
)

// KeyRotationScheduler 密钥轮换调度器
// 负责定期检查并执行密钥轮换
type KeyRotationScheduler struct {
	rotationApp *jwks.KeyRotationAppService
	logger      log.Logger

	checkInterval time.Duration // 检查间隔

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.RWMutex
	running bool
}

// NewKeyRotationScheduler 创建密钥轮换调度器
func NewKeyRotationScheduler(
	rotationApp *jwks.KeyRotationAppService,
	checkInterval time.Duration,
	logger log.Logger,
) *KeyRotationScheduler {
	if checkInterval == 0 {
		checkInterval = 1 * time.Hour // 默认每小时检查一次
	}

	return &KeyRotationScheduler{
		rotationApp:   rotationApp,
		logger:        logger,
		checkInterval: checkInterval,
	}
}

// Start 启动调度器
func (s *KeyRotationScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.logger.Warn("Key rotation scheduler is already running")
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	s.wg.Add(1)
	go s.run()

	s.logger.Infow("Key rotation scheduler started",
		"checkInterval", s.checkInterval,
	)

	return nil
}

// Stop 停止调度器
func (s *KeyRotationScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		s.logger.Warn("Key rotation scheduler is not running")
		return nil
	}

	s.logger.Info("Stopping key rotation scheduler...")

	s.cancel()
	s.wg.Wait()
	s.running = false

	s.logger.Info("Key rotation scheduler stopped")
	return nil
}

// IsRunning 返回调度器是否正在运行
func (s *KeyRotationScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// TriggerNow 立即触发一次密钥轮换检查
func (s *KeyRotationScheduler) TriggerNow(ctx context.Context) error {
	s.logger.Info("Manually triggering key rotation check")
	return s.checkAndRotate(ctx)
}

// run 调度器主循环
func (s *KeyRotationScheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	s.logger.Infow("Key rotation scheduler is running",
		"checkInterval", s.checkInterval,
	)

	// 首次启动时立即检查一次
	if err := s.checkAndRotate(s.ctx); err != nil {
		s.logger.Errorw("Initial key rotation check failed", "error", err)
	}

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Key rotation scheduler context cancelled")
			return
		case <-ticker.C:
			s.logger.Debug("Running scheduled key rotation check")
			if err := s.checkAndRotate(s.ctx); err != nil {
				s.logger.Errorw("Scheduled key rotation check failed", "error", err)
			}
		}
	}
}

// checkAndRotate 检查并执行密钥轮换
func (s *KeyRotationScheduler) checkAndRotate(ctx context.Context) error {
	// 检查是否需要轮换
	shouldRotateResp, err := s.rotationApp.ShouldRotate(ctx)
	if err != nil {
		s.logger.Errorw("Failed to check if rotation is needed", "error", err)
		return err
	}

	if !shouldRotateResp.ShouldRotate {
		s.logger.Debugw("Key rotation not needed", "reason", shouldRotateResp.Reason)
		return nil
	}

	// 执行轮换
	s.logger.Infow("Starting automatic key rotation", "reason", shouldRotateResp.Reason)

	resp, err := s.rotationApp.RotateKey(ctx)
	if err != nil {
		s.logger.Errorw("Automatic key rotation failed", "error", err)
		return err
	}

	s.logger.Infow("Automatic key rotation completed successfully",
		"newKid", resp.NewKey.Kid,
		"algorithm", resp.NewKey.Algorithm,
	)

	return nil
}
