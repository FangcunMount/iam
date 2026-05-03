package process

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/processruntime"
	"github.com/FangcunMount/component-base/pkg/shutdown"
)

// outboxRelayInterval 获取出box 间隔时间
func (s *apiServer) outboxRelayInterval() time.Duration {
	// 如果 API 服务器为空，则返回 2 秒
	if s == nil || s.cfg == nil || s.cfg.Events == nil {
		return 2 * time.Second
	}
	// 返回出box 间隔时间
	return s.cfg.Events.OutboxRelayInterval
}

// startRuntimeTasks 启动运行时任务
func (s *apiServer) startRuntimeTasks(lifecycle *processruntime.Lifecycle) {
	// 如果容器为空，则返回
	if s.container == nil {
		return
	}
	// 构建运行时依赖
	deps := s.container.BuildRuntimeDeps()
	// 定义停止旋转调度器函数
	var stopRotationScheduler func() error
	if deps.RotationScheduler != nil {
		// 获取旋转调度器
		scheduler := deps.RotationScheduler
		// 启动旋转调度器
		go func() {
			if err := scheduler.Start(context.Background()); err != nil {
				log.Errorf("failed to start key rotation scheduler: %v", err)
			}
		}()

		// 定义停止旋转调度器函数
		stopRotationScheduler = func() error {
			if scheduler.IsRunning() {
				// 停止旋转调度器
				return scheduler.Stop()
			}
			// 返回 nil
			return nil
		}
		// 记录旋转调度器初始化信息
		log.Infow("Key rotation scheduler initialized", "description", "periodic key rotation scheduler started")
	}
	// 获取出box 依赖
	if relay := deps.OutboxRelay; relay != nil {
		// 创建上下文
		ctx := context.Background()
		// 如果生命周期不为空，则添加关闭钩子
		if lifecycle != nil {
			// 创建取消函数
			var cancel context.CancelFunc
			// 创建取消上下文
			ctx, cancel = context.WithCancel(ctx)
			// 添加关闭钩子
			lifecycle.AddShutdownHook("stop outbox relay", func() error {
				// 取消上下文
				cancel()
				return nil
			})
		}
		// 启动出box 调度器
		go s.runOutboxRelay(ctx, relay)
		// 记录出box 调度器初始化信息
		log.Infow("Outbox relay initialized", "description", "domain event outbox relay started")
	}
	// 如果生命周期不为空，且停止旋转调度器不为空，则添加关闭钩子
	if lifecycle != nil && stopRotationScheduler != nil {
		// 添加关闭钩子
		lifecycle.AddShutdownHook("stop key rotation scheduler", stopRotationScheduler)
	}
}

// runOutboxRelay 运行出box 调度器
func (s *apiServer) runOutboxRelay(ctx context.Context, relay interface {
	DispatchDue(context.Context) error
}) {
	// 获取出box 间隔时间
	interval := s.outboxRelayInterval()
	// 如果间隔时间小于等于 0，则使用 2 秒
	if interval <= 0 {
		interval = 2 * time.Second
	}
	// 创建定时器
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// 调度出box
		if err := relay.DispatchDue(ctx); err != nil {
			log.Warnw("outbox relay dispatch failed", "error", err)
		}
		// 选择上下文
		select {
		case <-ctx.Done(): // 如果上下文完成，则返回
			return
		case <-ticker.C: // 如果定时器触发，则继续
		}
	}
}

// registerShutdownCallbacks 注册关闭回调
func (s *apiServer) registerShutdownCallbacks(lifecycle processruntime.Lifecycle) {
	s.gs.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
		runShutdownSequence(s.buildShutdownSequenceDeps(lifecycle))
		log.Info("🏗️  Hexagonal Architecture server shutdown complete")
		return nil
	}))
}

// shutdownSequenceDeps 关闭序列依赖
type shutdownSequenceDeps struct {
	lifecycle      processruntime.Lifecycle
	suggestCleanup func() error // 清理建议模块
	closeDatabase  func() error // 关闭数据库
	closeHTTP      func() error // 关闭 HTTP 服务器
	closeGRPC      func() error // 关闭 GRPC 服务器
}

// buildShutdownSequenceDeps 构建关闭序列依赖
func (s *apiServer) buildShutdownSequenceDeps(lifecycle processruntime.Lifecycle) shutdownSequenceDeps {
	deps := shutdownSequenceDeps{lifecycle: lifecycle}
	if s == nil { // 如果 API 服务器为空，则返回
		return deps
	}
	if s.container != nil {
		deps.suggestCleanup = s.container.BuildRuntimeDeps().SuggestCleanup // 构建建议模块依赖
	}
	if s.dbManager != nil {
		deps.closeDatabase = s.dbManager.Close // 关闭数据库
	}
	if s.genericAPIServer != nil {
		deps.closeHTTP = func() error {
			s.genericAPIServer.Close() // 关闭 HTTP 服务器
			return nil
		}
	}
	if s.grpcServer != nil {
		deps.closeGRPC = func() error {
			s.grpcServer.Close() // 关闭 GRPC 服务器
			return nil
		}
	}
	// 返回关闭序列依赖
	return deps
}

// runShutdownSequence 运行关闭序列
func runShutdownSequence(deps shutdownSequenceDeps) {
	// 运行生命周期
	deps.lifecycle.Run(func(name string, err error) {
		log.Errorf("Failed to run shutdown hook %q: %v", name, err)
	})
	// 运行关闭序列步骤
	runShutdownStep("cleanup suggest module", deps.suggestCleanup)
	runShutdownStep("close database connections", deps.closeDatabase)
	runShutdownStep("close HTTP server", deps.closeHTTP) // 运行关闭 HTTP 服务器
	runShutdownStep("close gRPC server", deps.closeGRPC) // 运行关闭 GRPC 服务器
}

// runShutdownStep 运行关闭序列步骤
func runShutdownStep(name string, run func() error) {
	if run == nil { // 如果运行函数为空，则返回
		return
	}
	if err := run(); err != nil { // 如果运行函数返回错误，则记录错误
		log.Errorf("Failed to %s: %v", name, err)
	}
}
