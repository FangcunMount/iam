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
	if err := runPreparedServerGroup(deps); err != nil {
		log.Errorf("Failed to run prepared server: %v", err)
		return err
	}
	return nil
}

func runPreparedServerGroup(deps preparedServerRunDeps) error {
	if deps.startShutdown != nil {
		if err := deps.startShutdown(); err != nil {
			return err
		}
	}

	return runPreparedServerTransports(deps.transports)
}

func runPreparedServerTransports(transports preparedServerTransports) error {
	services := []processruntime.ServiceRunner{
		{Name: "http", Run: transports.runHTTP},
		{Name: "grpc", Run: transports.runGRPC},
	}

	active := 0
	for _, service := range services {
		if service.Run != nil {
			active++
		}
	}
	if active == 0 {
		return nil
	}

	releaseErrors := make(chan struct{})
	observedErrors := make(chan error, active)
	for i := range services {
		services[i].Run = observeServiceError(services[i].Run, observedErrors, releaseErrors)
	}

	groupDone := make(chan error, 1)
	go func() {
		groupDone <- (processruntime.RunGroup{Services: services}).Run()
	}()

	select {
	case err := <-observedErrors:
		close(releaseErrors)
		if groupErr := <-groupDone; groupErr != nil {
			return groupErr
		}
		return err
	case err := <-groupDone:
		close(releaseErrors)
		return err
	}
}

func observeServiceError(run func() error, observedErrors chan<- error, releaseErrors <-chan struct{}) func() error {
	if run == nil {
		return nil
	}
	return func() error {
		err := run()
		if err == nil {
			return nil
		}
		// component-base v0.6.1 can randomly choose its done branch when a
		// short-lived service returns an error and all services have exited.
		// Hold the service goroutine until this package has observed the error.
		observedErrors <- err
		<-releaseErrors
		return err
	}
}

// Run 运行 API 服务器。
func (s preparedAPIServer) Run() error {
	return runPreparedServer(s.buildPreparedServerRunDeps())
}
