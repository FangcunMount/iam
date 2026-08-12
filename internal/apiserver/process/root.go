package process

import (
	"github.com/FangcunMount/component-base/pkg/shutdown"
	"github.com/FangcunMount/component-base/pkg/shutdown/shutdownmanagers/posixsignal"
	"github.com/FangcunMount/iam/v3/internal/apiserver/config"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container"
	"github.com/FangcunMount/iam/v3/internal/pkg/grpc"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
)

// apiServer 定义了 API 服务器的基本结构（六边形架构版本）
type apiServer struct {
	cfg              *config.Config                     // APIServer 配置
	gs               *shutdown.GracefulShutdown         // GracefulShutdown 优雅关闭管理器
	genericAPIServer *genericapiserver.GenericAPIServer // 通用 API 服务器
	grpcServer       *grpc.Server                       // GRPC 服务器
	dbManager        *DatabaseManager                   // 数据库管理器
	container        *container.Container               // Container 主容器
}

// preparedAPIServer 定义了准备运行的 APIServer
type preparedAPIServer struct {
	*apiServer
}

// createAPIServer 创建 APIServer 实例（六边形架构版本）
func createAPIServer(cfg *config.Config) (*apiServer, error) {
	// 创建一个 GracefulShutdown 实例
	gs := shutdown.New()
	// 添加 POSIX signal 关闭管理器
	gs.AddShutdownManager(posixsignal.NewPosixSignalManager())

	// 创建 DatabaseManager 数据库管理器
	dbManager := NewDatabaseManager(cfg)

	// 创建 apiServer 实例
	server := &apiServer{
		cfg:       cfg,
		gs:        gs,
		dbManager: dbManager,
	}

	return server, nil
}

// PrepareRun 准备运行 APIServer（六边形架构版本）
func (s *apiServer) PrepareRun() (preparedAPIServer, error) {
	// 创建准备运行 APIServer
	prepared, _, err := newPrepareRunner(s).run()
	if err != nil {
		// 如果准备运行 APIServer 失败，则返回错误
		return preparedAPIServer{}, err
	}

	// 返回准备运行 APIServer
	return prepared, nil
}
