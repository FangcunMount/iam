package process

import (
	"fmt"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/component-base/pkg/processruntime"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
	grpctransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc"
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
	grpcpkg "github.com/FangcunMount/iam/v2/internal/pkg/grpc"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// runtimeOutput 运行时输出
type runtimeOutput struct {
	mode            string                   // 运行时模式
	appMode         string                   // 应用模式
	degradedAllowed bool                     // 是否允许降级启动
	lifecycle       processruntime.Lifecycle // 生命周期
}

// resourceOutput 资源输出
type resourceOutput struct {
	mysqlDB          *gorm.DB           // MySQL 数据库
	cacheClient      *redis.Client      // Redis 缓存客户端
	idpEncryptionKey []byte             // IDP 加密密钥
	eventBus         messaging.EventBus // 事件总线
}

// containerOutput 容器输出
type containerOutput struct {
	container *container.Container // 容器
}

// transportOutput 传输输出
type transportOutput struct {
	httpServer *genericapiserver.GenericAPIServer // HTTP 服务器
	grpcServer *grpcpkg.Server                    // GRPC 服务器
}

// transportStageDeps 传输阶段依赖
type transportStageDeps struct {
	buildHTTPServer func() (*genericapiserver.GenericAPIServer, error) // 构建 HTTP 服务器
	buildGRPCServer func() (*grpcpkg.Server, error)                    // 构建 GRPC 服务器
	registerREST    func(*genericapiserver.GenericAPIServer)           // 注册 REST 路由
	registerGRPC    func(*grpcpkg.Server) error                        // 注册 GRPC 服务
}

// prepareRuntime 准备运行时
func (s *apiServer) prepareRuntime() runtimeOutput {
	// 获取运行时模式
	mode := runtimeMode(s.cfg)
	appMode := appModeFromServerMode(mode)
	// 返回运行时输出
	return runtimeOutput{
		mode:            mode,
		appMode:         appMode,
		degradedAllowed: degradedStartupAllowed(s.cfg),
	}
}

// prepareResources 准备资源
func (s *apiServer) prepareResources(rt runtimeOutput) (resourceOutput, error) {
	// 初始化数据库
	if err := s.dbManager.Initialize(); err != nil {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("initialize database: %w", err)
		}
		log.Warnw("degraded startup: database initialization failed", "error", err, "mode", rt.mode)
	}

	// 获取 MySQL 数据库
	mysqlDB, err := s.dbManager.GetMySQLDB()
	if err != nil {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("mysql unavailable: %w", err)
		}
		log.Warnw("degraded startup: MySQL unavailable", "error", err, "mode", rt.mode)
		mysqlDB = nil
	}

	// 获取 Redis 缓存客户端
	cacheClient, err := s.dbManager.GetCacheRedisClient()
	if err != nil {
		if !rt.degradedAllowed {
			return resourceOutput{}, fmt.Errorf("cache redis unavailable: %w", err)
		}
		log.Warnw("degraded startup: cache redis unavailable", "error", err, "mode", rt.mode)
		cacheClient = nil
	}

	// 解析 IDP 加密密钥
	idpEncryptionKey, configured, err := parseIDPEncryptionKey(s.idpEncryptionSecret())
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

	// 创建事件总线
	eventBus, err := s.createEventBus()
	if err != nil {
		log.Warnw("event bus unavailable; continue without notifier", "error", err)
		eventBus = nil
	}

	// 返回资源输出
	return resourceOutput{
		mysqlDB:          mysqlDB,
		cacheClient:      cacheClient,
		idpEncryptionKey: idpEncryptionKey,
		eventBus:         eventBus,
	}, nil
}

// prepareContainer 准备容器
func (s *apiServer) prepareContainer(rt runtimeOutput, resources resourceOutput) (containerOutput, error) {
	// 创建容器
	s.container = container.NewContainerWithOptions(
		resources.mysqlDB,
		resources.cacheClient,
		resources.eventBus,
		resources.idpEncryptionKey,
		container.RuntimeOptionsFromAPIServerOptions(s.cfg.Options, rt.appMode),
	)

	// 初始化容器
	if err := s.container.Initialize(); err != nil {
		if !rt.degradedAllowed {
			return containerOutput{}, fmt.Errorf("initialize container: %w", err)
		}
		log.Warnw("degraded startup: container initialization incomplete", "error", err, "mode", rt.mode)
	}

	// 验证关键模块
	if err := s.validateCriticalModules(rt.degradedAllowed); err != nil {
		return containerOutput{}, err
	}

	// 返回容器输出
	return containerOutput{container: s.container}, nil
}

// prepareTransports 准备传输
func (s *apiServer) prepareTransports(rt runtimeOutput, out containerOutput) (transportOutput, error) {
	// 构建传输阶段依赖
	transport, err := bootstrapTransports(s.buildTransportStageDeps(rt, out))
	if err != nil {
		return transportOutput{}, err
	}
	// 设置 HTTP 服务器
	s.genericAPIServer = transport.httpServer
	// 设置 GRPC 服务器
	s.grpcServer = transport.grpcServer
	// 返回传输输出
	return transport, nil
}

// buildTransportStageDeps 构建传输阶段依赖
func (s *apiServer) buildTransportStageDeps(rt runtimeOutput, out containerOutput) transportStageDeps {
	// 如果 API 服务器为空，则返回空传输阶段依赖
	if s == nil || s.cfg == nil || out.container == nil {
		return transportStageDeps{}
	}
	// 返回传输阶段依赖
	return transportStageDeps{
		buildHTTPServer: func() (*genericapiserver.GenericAPIServer, error) {
			return buildGenericServer(s.cfg)
		},
		buildGRPCServer: func() (*grpcpkg.Server, error) {
			return buildGRPCServer(s.cfg)
		},
		registerREST: func(httpServer *genericapiserver.GenericAPIServer) {
			resttransport.NewRouter(out.container.BuildRESTDeps(routerOptionsFromConfig(s.cfg.Options, rt.appMode))).RegisterRoutes(httpServer.Engine)
		},
		registerGRPC: func(grpcServer *grpcpkg.Server) error {
			return grpctransport.NewRegistry(out.container.BuildGRPCDeps(grpcServer)).RegisterServices()
		},
	}
}

// bootstrapTransports 引导传输
func bootstrapTransports(deps transportStageDeps) (transportOutput, error) {
	// 创建传输输出
	var output transportOutput
	if deps.buildHTTPServer != nil {
		// 构建 HTTP 服务器
		httpServer, err := deps.buildHTTPServer()
		if err != nil {
			return transportOutput{}, err
		}
		output.httpServer = httpServer
	}
	if deps.buildGRPCServer != nil {
		// 构建 GRPC 服务器
		grpcServer, err := deps.buildGRPCServer()
		if err != nil {
			return transportOutput{}, err
		}
		output.grpcServer = grpcServer
	}
	if deps.registerREST != nil && output.httpServer != nil {
		// 注册 REST 路由
		deps.registerREST(output.httpServer)
	}
	if deps.registerGRPC != nil && output.grpcServer != nil {
		// 注册 GRPC 服务
		if err := deps.registerGRPC(output.grpcServer); err != nil {
			return transportOutput{}, err
		}
	}
	// 返回传输输出
	return output, nil
}

// routerOptionsFromConfig 从配置中获取路由选项
func routerOptionsFromConfig(opts *apiserveroptions.Options, appMode string) resttransport.RouterOptions {
	// 获取种子模拟认证选项
	var seed apiserveroptions.SeedMockAuthOptions
	// 获取调试缓存治理选项
	var debug apiserveroptions.DebugOptions
	if opts != nil && opts.SeedMockAuth != nil {
		seed = *opts.SeedMockAuth
	}
	if opts != nil && opts.Debug != nil {
		debug = *opts.Debug
	}
	// 创建路由选项
	options := resttransport.RouterOptions{
		DebugCacheGovernance: resttransport.DebugCacheGovernanceOptions{
			AppMode: appMode,
		},
		SeedMockAuth: resttransport.SeedMockAuthOptions{
			Enabled:      seed.Enabled,
			SharedSecret: seed.SharedSecret,
		},
	}

	// 设置调试缓存治理选项
	options.DebugCacheGovernance.Enabled = debug.CacheGovernance.Enabled
	options.DebugCacheGovernance.RequireAdmin = debug.CacheGovernance.RequireAdmin

	// 返回路由选项
	return options
}

// idpEncryptionSecret 获取 IDP 加密密钥
func (s *apiServer) idpEncryptionSecret() string {
	// 如果 API 服务器为空，则返回空字符串
	if s == nil || s.cfg == nil || s.cfg.IDP == nil {
		return ""
	}
	// 返回 IDP 加密密钥
	return s.cfg.IDP.EncryptionKey
}
