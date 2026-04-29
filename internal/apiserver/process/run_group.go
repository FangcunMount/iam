package process

import (
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/processruntime"
)

type preparedServerTransports struct {
	runHTTP func() error
	runGRPC func() error
}

type preparedServerRunDeps struct {
	startShutdown func() error
	transports    preparedServerTransports
}

func (s preparedAPIServer) buildPreparedServerRunDeps() preparedServerRunDeps {
	var deps preparedServerRunDeps
	if s.gs != nil {
		deps.startShutdown = s.gs.Start
	}
	if s.genericAPIServer != nil {
		deps.transports.runHTTP = s.genericAPIServer.Run
	}
	if s.grpcServer != nil {
		deps.transports.runGRPC = s.grpcServer.Run
	}
	return deps
}

func runPreparedServer(deps preparedServerRunDeps) error {
	if deps.transports.runHTTP != nil {
		log.Info("🚀 Starting Hexagonal Architecture HTTP REST API server...")
	}
	if deps.transports.runGRPC != nil {
		log.Info("🚀 Starting Hexagonal Architecture GRPC server...")
	}
	if err := (processruntime.RunGroup{
		StartShutdown: deps.startShutdown,
		Services: []processruntime.ServiceRunner{
			{Name: "http", Run: deps.transports.runHTTP},
			{Name: "grpc", Run: deps.transports.runGRPC},
		},
	}).Run(); err != nil {
		log.Errorf("Failed to run prepared server: %v", err)
		return err
	}
	return nil
}

// Run 运行 API 服务器。
func (s preparedAPIServer) Run() error {
	return runPreparedServer(s.buildPreparedServerRunDeps())
}
