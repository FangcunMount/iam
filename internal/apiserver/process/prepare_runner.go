package process

import (
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/processruntime"
)

// prepareState 准备状态
type prepareState struct {
	runtime   runtimeOutput   // 运行时输出
	resources resourceOutput  // 资源输出
	container containerOutput // 容器输出
	transport transportOutput // 传输输出
}

// prepareRunner 准备运行器
type prepareRunner struct {
	server *apiServer                           // APIServer
	state  prepareState                         // 准备状态
	stages []processruntime.Stage[prepareState] // 阶段
}

// newPrepareRunner 创建准备运行器
func newPrepareRunner(server *apiServer) *prepareRunner {
	return &prepareRunner{
		server: server,
		stages: []processruntime.Stage[prepareState]{
			prepareRuntimeStage{server: server}, // 准备运行时
			resourceStage{server: server},       // 准备资源
			containerStage{server: server},      // 准备容器
			transportStage{server: server},      // 准备传输
			runtimeTaskStage{server: server},    // 准备运行时任务
			shutdownStage{server: server},       // 准备关闭回调
		},
	}
}

// run 运行准备运行器
func (r *prepareRunner) run() (preparedAPIServer, string, error) {
	return processruntime.Runner[prepareState, preparedAPIServer]{
		State:  &r.state,
		Stages: r.stages,
		BuildPrepared: func(*prepareState) preparedAPIServer {
			return preparedAPIServer{r.server}
		},
	}.Run()
}

// prepareRuntimeStage 准备运行时阶段
type prepareRuntimeStage struct {
	server *apiServer
}

// Name 返回准备运行时阶段名称
func (prepareRuntimeStage) Name() string { return "prepare runtime" }

// Run 运行准备运行时阶段
func (s prepareRuntimeStage) Run(state *prepareState) error {
	runtime, err := s.server.prepareRuntime()
	if err != nil {
		return err
	}
	state.runtime = runtime
	return nil
}

// resourceStage 准备资源阶段
type resourceStage struct {
	server *apiServer
}

// Name 返回准备资源阶段名称
func (resourceStage) Name() string { return "prepare resources" }

// Run 运行准备资源阶段
func (s resourceStage) Run(state *prepareState) error {
	resources, err := s.server.prepareResources(state.runtime)
	if err != nil {
		return err
	}
	state.resources = resources
	return nil
}

// containerStage 准备容器阶段
type containerStage struct {
	server *apiServer
}

// Name 返回准备容器阶段名称
func (containerStage) Name() string { return "initialize container" }

// Run 运行准备容器阶段
func (s containerStage) Run(state *prepareState) error {
	containerOut, err := s.server.prepareContainer(state.runtime, state.resources)
	if err != nil {
		return err
	}
	state.container = containerOut
	return nil
}

// transportStage 准备传输阶段
type transportStage struct {
	server *apiServer
}

// Name 返回准备传输阶段名称
func (transportStage) Name() string { return "initialize transports" }

// Run 运行准备传输阶段
func (s transportStage) Run(state *prepareState) error {
	transportOut, err := s.server.prepareTransports(state.runtime, state.container)
	if err != nil {
		return err
	}
	state.transport = transportOut
	return nil
}

// runtimeTaskStage 准备运行时任务阶段
type runtimeTaskStage struct {
	server *apiServer
}

// Name 返回准备运行时任务阶段名称
func (runtimeTaskStage) Name() string { return "start runtime tasks" }

// Run 运行准备运行时任务阶段
func (s runtimeTaskStage) Run(state *prepareState) error {
	s.server.startRuntimeTasks(&state.runtime.lifecycle)
	log.Infow("hexagonal architecture initialized",
		"server_mode", state.runtime.profile.ServerMode,
		"environment", state.runtime.profile.Environment,
		"production_like", state.runtime.profile.IsProductionLike(),
		"degraded_startup_allowed", state.runtime.degradedAllowed,
	)
	return nil
}

// shutdownStage 准备关闭回调阶段
type shutdownStage struct {
	server *apiServer
}

// Name 返回准备关闭回调阶段名称
func (shutdownStage) Name() string { return "register shutdown callbacks" }

// Run 运行准备关闭回调阶段
func (s shutdownStage) Run(state *prepareState) error {
	s.server.registerShutdownCallbacks(state.runtime.lifecycle)
	return nil
}
