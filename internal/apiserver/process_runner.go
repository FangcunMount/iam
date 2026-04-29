package apiserver

import (
	"github.com/FangcunMount/component-base/pkg/log"
	apiserverprocess "github.com/FangcunMount/iam/internal/apiserver/process"
)

type prepareState struct {
	runtime   runtimeOutput
	resources resourceOutput
	container containerOutput
}

type prepareRunner struct {
	server *apiServer
	state  prepareState
	stages []apiserverprocess.Stage[prepareState]
}

func newPrepareRunner(server *apiServer) *prepareRunner {
	return &prepareRunner{
		server: server,
		stages: []apiserverprocess.Stage[prepareState]{
			prepareRuntimeStage{server: server},
			resourceStage{server: server},
			containerStage{server: server},
			transportStage{server: server},
			runtimeTaskStage{server: server},
			shutdownStage{server: server},
		},
	}
}

func (r *prepareRunner) run() (preparedAPIServer, string, error) {
	return apiserverprocess.Runner[prepareState, preparedAPIServer]{
		State:  &r.state,
		Stages: r.stages,
		BuildPrepared: func(*prepareState) preparedAPIServer {
			return preparedAPIServer{r.server}
		},
	}.Run()
}

type prepareRuntimeStage struct {
	server *apiServer
}

func (prepareRuntimeStage) Name() string { return "prepare runtime" }

func (s prepareRuntimeStage) Run(state *prepareState) error {
	state.runtime = s.server.prepareRuntime()
	return nil
}

type resourceStage struct {
	server *apiServer
}

func (resourceStage) Name() string { return "prepare resources" }

func (s resourceStage) Run(state *prepareState) error {
	resources, err := s.server.prepareResources(state.runtime)
	if err != nil {
		return err
	}
	state.resources = resources
	return nil
}

type containerStage struct {
	server *apiServer
}

func (containerStage) Name() string { return "initialize container" }

func (s containerStage) Run(state *prepareState) error {
	containerOut, err := s.server.prepareContainer(state.runtime, state.resources)
	if err != nil {
		return err
	}
	state.container = containerOut
	return nil
}

type transportStage struct {
	server *apiServer
}

func (transportStage) Name() string { return "initialize transports" }

func (s transportStage) Run(state *prepareState) error {
	s.server.prepareTransports(state.runtime, state.container)
	return nil
}

type runtimeTaskStage struct {
	server *apiServer
}

func (runtimeTaskStage) Name() string { return "start runtime tasks" }

func (s runtimeTaskStage) Run(state *prepareState) error {
	s.server.startRuntimeTasks()
	log.Infow("hexagonal architecture initialized",
		"mode", state.runtime.mode,
		"degraded_startup_allowed", state.runtime.degradedAllowed,
	)
	return nil
}

type shutdownStage struct {
	server *apiServer
}

func (shutdownStage) Name() string { return "register shutdown callbacks" }

func (s shutdownStage) Run(*prepareState) error {
	s.server.registerShutdownCallbacks()
	return nil
}
