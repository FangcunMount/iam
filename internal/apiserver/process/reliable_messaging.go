package process

import (
	"context"
	"os"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/shutdown"
)

func (s *apiServer) reliableMessagingEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Options != nil && s.cfg.Events != nil && s.cfg.Events.ReliableMessaging.Enabled
}

// component-base's POSIX manager otherwise exits 0 after callback errors. A
// failed reliable drain must be observable as a failed process termination;
// its leased records stay recoverable. This is not a graceful completion claim.
func configureReliableShutdownFailure(gs *shutdown.GracefulShutdown, enabled bool) {
	if !enabled {
		return
	}
	gs.SetErrorHandler(shutdown.ErrorFunc(func(error) {
		log.Errorw("reliable messaging shutdown failed; terminating without successful drain", "exit_code", 1)
		log.Flush()
		os.Exit(1)
	}))
}

func (s *apiServer) cleanupReliableStartup() {
	if s == nil || s.container == nil {
		return
	}
	deps := s.container.BuildRuntimeDeps()
	if deps.ReliableMessaging == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), deps.ReliableShutdownTimeout)
	defer cancel()
	if err := deps.ReliableMessaging.Stop(ctx); err != nil {
		log.Errorw("failed startup reliable messaging drain incomplete", "stage", "startup_cleanup")
		return
	}
	if deps.CloseReliableProducer != nil {
		deps.CloseReliableProducer()
	}
}
