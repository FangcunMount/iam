package apiserver

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/component-base/pkg/shutdown"
	"github.com/FangcunMount/iam/internal/apiserver/container"
	redis "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type runtimeOutput struct {
	mode            string
	degradedAllowed bool
}

type resourceOutput struct {
	mysqlDB          *gorm.DB
	cacheClient      *redis.Client
	idpEncryptionKey []byte
	eventBus         messaging.EventBus
}

type containerOutput struct {
	container *container.Container
}

func (s *apiServer) prepareRuntime() runtimeOutput {
	mode := runtimeMode(s.cfg)
	viper.Set("app.mode", appModeFromServerMode(mode))
	return runtimeOutput{
		mode:            mode,
		degradedAllowed: degradedStartupAllowed(s.cfg),
	}
}

func (s *apiServer) prepareResources(rt runtimeOutput) (resourceOutput, error) {
	if err := s.dbManager.Initialize(); err != nil {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("initialize database: %w", err)
		}
		log.Warnw("degraded startup: database initialization failed", "error", err, "mode", rt.mode)
	}

	mysqlDB, err := s.dbManager.GetMySQLDB()
	if err != nil {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("mysql unavailable: %w", err)
		}
		log.Warnw("degraded startup: MySQL unavailable", "error", err, "mode", rt.mode)
		mysqlDB = nil
	}

	cacheClient, err := s.dbManager.GetCacheRedisClient()
	if err != nil {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("cache redis unavailable: %w", err)
		}
		log.Warnw("degraded startup: cache redis unavailable", "error", err, "mode", rt.mode)
		cacheClient = nil
	}

	idpEncryptionKey, configured, err := loadIDPEncryptionKey()
	if err != nil {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("parse idp encryption key: %w", err)
		}
		log.Warnw("degraded startup: invalid idp encryption key", "error", err, "mode", rt.mode)
	}
	if !configured {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("idp.encryption-key is required")
		}
		log.Warnw("degraded startup: idp.encryption-key missing", "mode", rt.mode)
	}

	eventBus, err := s.createEventBus()
	if err != nil {
		log.Warnw("event bus unavailable; continue without notifier", "error", err)
		eventBus = nil
	}

	return resourceOutput{
		mysqlDB:          mysqlDB,
		cacheClient:      cacheClient,
		idpEncryptionKey: idpEncryptionKey,
		eventBus:         eventBus,
	}, nil
}

func (s *apiServer) prepareContainer(rt runtimeOutput, resources resourceOutput) (containerOutput, error) {
	s.container = container.NewContainer(
		resources.mysqlDB,
		resources.cacheClient,
		resources.eventBus,
		resources.idpEncryptionKey,
	)

	if err := s.container.Initialize(); err != nil {
		if !rt.degradedAllowed {
			return containerOutput{}, fmt.Errorf("initialize container: %w", err)
		}
		log.Warnw("degraded startup: container initialization incomplete", "error", err, "mode", rt.mode)
	}

	if err := s.validateCriticalModules(rt.degradedAllowed); err != nil {
		return containerOutput{}, err
	}

	return containerOutput{container: s.container}, nil
}

func (s *apiServer) prepareTransports(out containerOutput) {
	NewRouter(out.container).RegisterRoutes(s.genericAPIServer.Engine)
	s.registerGRPCServices()
}

func (s *apiServer) startRuntimeTasks() {
	if s.container == nil {
		return
	}
	if s.container.AuthnModule != nil && s.container.AuthnModule.RotationScheduler != nil {
		go func() {
			if err := s.container.AuthnModule.RotationScheduler.Start(context.Background()); err != nil {
				log.Errorf("failed to start key rotation scheduler: %v", err)
			}
		}()
		log.Infow("Key rotation scheduler initialized", "description", "periodic key rotation scheduler started")
	}
	if relay := s.container.OutboxRelay(); relay != nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.outboxRelayCancel = cancel
		go s.runOutboxRelay(ctx, relay)
		log.Infow("Outbox relay initialized", "description", "domain event outbox relay started")
	}
}

func (s *apiServer) runOutboxRelay(ctx context.Context, relay interface {
	DispatchDue(context.Context) error
}) {
	interval := viper.GetDuration("events.outbox_relay_interval")
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

func (s *apiServer) registerShutdownCallbacks() {
	s.gs.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
		if s.outboxRelayCancel != nil {
			s.outboxRelayCancel()
			s.outboxRelayCancel = nil
		}

		if s.container != nil && s.container.AuthnModule != nil && s.container.AuthnModule.RotationScheduler != nil && s.container.AuthnModule.RotationScheduler.IsRunning() {
			if err := s.container.AuthnModule.RotationScheduler.Stop(); err != nil {
				log.Errorf("Failed to stop key rotation scheduler: %v", err)
			}
		}

		if s.container != nil && s.container.SuggestModule != nil {
			if err := s.container.SuggestModule.Cleanup(); err != nil {
				log.Errorf("Failed to cleanup suggest module: %v", err)
			}
		}

		if s.dbManager != nil {
			if err := s.dbManager.Close(); err != nil {
				log.Errorf("Failed to close database connections: %v", err)
			}
		}

		s.genericAPIServer.Close()
		s.grpcServer.Close()

		log.Info("🏗️  Hexagonal Architecture server shutdown complete")
		return nil
	}))
}
