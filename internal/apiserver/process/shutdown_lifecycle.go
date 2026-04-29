package process

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/processruntime"
	"github.com/FangcunMount/component-base/pkg/shutdown"
)

func (s *apiServer) outboxRelayInterval() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.Events == nil {
		return 2 * time.Second
	}
	return s.cfg.Events.OutboxRelayInterval
}

func (s *apiServer) startRuntimeTasks(lifecycle *processruntime.Lifecycle) {
	if s.container == nil {
		return
	}
	deps := s.container.BuildRuntimeDeps()
	var stopRotationScheduler func() error
	if deps.RotationScheduler != nil {
		scheduler := deps.RotationScheduler
		go func() {
			if err := scheduler.Start(context.Background()); err != nil {
				log.Errorf("failed to start key rotation scheduler: %v", err)
			}
		}()
		stopRotationScheduler = func() error {
			if scheduler.IsRunning() {
				return scheduler.Stop()
			}
			return nil
		}
		log.Infow("Key rotation scheduler initialized", "description", "periodic key rotation scheduler started")
	}
	if relay := deps.OutboxRelay; relay != nil {
		ctx, cancel := context.WithCancel(context.Background())
		if lifecycle != nil {
			lifecycle.AddShutdownHook("stop outbox relay", func() error {
				cancel()
				return nil
			})
		}
		go s.runOutboxRelay(ctx, relay)
		log.Infow("Outbox relay initialized", "description", "domain event outbox relay started")
	}
	if lifecycle != nil && stopRotationScheduler != nil {
		lifecycle.AddShutdownHook("stop key rotation scheduler", stopRotationScheduler)
	}
}

func (s *apiServer) runOutboxRelay(ctx context.Context, relay interface {
	DispatchDue(context.Context) error
}) {
	interval := s.outboxRelayInterval()
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := relay.DispatchDue(ctx); err != nil {
			log.Warnw("outbox relay dispatch failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *apiServer) registerShutdownCallbacks(lifecycle processruntime.Lifecycle) {
	s.gs.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
		runShutdownSequence(s.buildShutdownSequenceDeps(lifecycle))
		log.Info("🏗️  Hexagonal Architecture server shutdown complete")
		return nil
	}))
}

type shutdownSequenceDeps struct {
	lifecycle      processruntime.Lifecycle
	suggestCleanup func() error
	closeDatabase  func() error
	closeHTTP      func() error
	closeGRPC      func() error
}

func (s *apiServer) buildShutdownSequenceDeps(lifecycle processruntime.Lifecycle) shutdownSequenceDeps {
	deps := shutdownSequenceDeps{lifecycle: lifecycle}
	if s == nil {
		return deps
	}
	if s.container != nil {
		deps.suggestCleanup = s.container.BuildRuntimeDeps().SuggestCleanup
	}
	if s.dbManager != nil {
		deps.closeDatabase = s.dbManager.Close
	}
	if s.genericAPIServer != nil {
		deps.closeHTTP = func() error {
			s.genericAPIServer.Close()
			return nil
		}
	}
	if s.grpcServer != nil {
		deps.closeGRPC = func() error {
			s.grpcServer.Close()
			return nil
		}
	}
	return deps
}

func runShutdownSequence(deps shutdownSequenceDeps) {
	deps.lifecycle.Run(func(name string, err error) {
		log.Errorf("Failed to run shutdown hook %q: %v", name, err)
	})
	runShutdownStep("cleanup suggest module", deps.suggestCleanup)
	runShutdownStep("close database connections", deps.closeDatabase)
	runShutdownStep("close HTTP server", deps.closeHTTP)
	runShutdownStep("close gRPC server", deps.closeGRPC)
}

func runShutdownStep(name string, run func() error) {
	if run == nil {
		return
	}
	if err := run(); err != nil {
		log.Errorf("Failed to %s: %v", name, err)
	}
}
