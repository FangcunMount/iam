package process

import (
	"github.com/FangcunMount/component-base/pkg/shutdown"
	"github.com/FangcunMount/component-base/pkg/shutdown/shutdownmanagers/posixsignal"
	"github.com/FangcunMount/iam/v2/internal/apiserver/config"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container"
	"github.com/FangcunMount/iam/v2/internal/pkg/grpc"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
)

// apiServer 定义了 API 服务器的基本结构（六边形架构版本）
type apiServer struct {
	cfg *config.Config
	// 优雅关闭管理器
	gs *shutdown.GracefulShutdown
	// 通用 API 服务器
	genericAPIServer *genericapiserver.GenericAPIServer
	// GRPC 服务器
	grpcServer *grpc.Server
	// 数据库管理器
	dbManager *DatabaseManager
	// Container 主容器
	container *container.Container
}

// preparedAPIServer 定义了准备运行的 API 服务器
type preparedAPIServer struct {
	*apiServer
}

// createAPIServer 创建 API 服务器实例（六边形架构版本）
func createAPIServer(cfg *config.Config) (*apiServer, error) {
	// 创建一个 GracefulShutdown 实例
	gs := shutdown.New()
	gs.AddShutdownManager(posixsignal.NewPosixSignalManager())

	// 创建数据库管理器
	dbManager := NewDatabaseManager(cfg)

	// 创建 API 服务器实例
	server := &apiServer{
		cfg:       cfg,
		gs:        gs,
		dbManager: dbManager,
	}

	return server, nil
}

// PrepareRun 准备运行 API 服务器（六边形架构版本）
func (s *apiServer) PrepareRun() (preparedAPIServer, error) {
	prepared, _, err := newPrepareRunner(s).run()
	if err != nil {
		return preparedAPIServer{}, err
	}
	return prepared, nil
}
