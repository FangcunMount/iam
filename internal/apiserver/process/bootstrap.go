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

type runtimeOutput struct {
	mode            string
	appMode         string
	degradedAllowed bool
	lifecycle       processruntime.Lifecycle
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

type transportOutput struct {
	httpServer *genericapiserver.GenericAPIServer
	grpcServer *grpcpkg.Server
}

type transportStageDeps struct {
	buildHTTPServer func() (*genericapiserver.GenericAPIServer, error)
	buildGRPCServer func() (*grpcpkg.Server, error)
	registerREST    func(*genericapiserver.GenericAPIServer)
	registerGRPC    func(*grpcpkg.Server) error
}

func (s *apiServer) prepareRuntime() runtimeOutput {
	mode := runtimeMode(s.cfg)
	appMode := appModeFromServerMode(mode)
	return runtimeOutput{
		mode:            mode,
		appMode:         appMode,
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
	s.container = container.NewContainerWithOptions(
		resources.mysqlDB,
		resources.cacheClient,
		resources.eventBus,
		resources.idpEncryptionKey,
		container.RuntimeOptionsFromAPIServerOptions(s.cfg.Options, rt.appMode),
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

func (s *apiServer) prepareTransports(rt runtimeOutput, out containerOutput) (transportOutput, error) {
	transport, err := bootstrapTransports(s.buildTransportStageDeps(rt, out))
	if err != nil {
		return transportOutput{}, err
	}
	s.genericAPIServer = transport.httpServer
	s.grpcServer = transport.grpcServer
	return transport, nil
}

func (s *apiServer) buildTransportStageDeps(rt runtimeOutput, out containerOutput) transportStageDeps {
	if s == nil || s.cfg == nil || out.container == nil {
		return transportStageDeps{}
	}
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

func bootstrapTransports(deps transportStageDeps) (transportOutput, error) {
	var output transportOutput
	if deps.buildHTTPServer != nil {
		httpServer, err := deps.buildHTTPServer()
		if err != nil {
			return transportOutput{}, err
		}
		output.httpServer = httpServer
	}
	if deps.buildGRPCServer != nil {
		grpcServer, err := deps.buildGRPCServer()
		if err != nil {
			return transportOutput{}, err
		}
		output.grpcServer = grpcServer
	}
	if deps.registerREST != nil && output.httpServer != nil {
		deps.registerREST(output.httpServer)
	}
	if deps.registerGRPC != nil && output.grpcServer != nil {
		if err := deps.registerGRPC(output.grpcServer); err != nil {
			return transportOutput{}, err
		}
	}
	return output, nil
}

func routerOptionsFromConfig(opts *apiserveroptions.Options, appMode string) resttransport.RouterOptions {
	var seed apiserveroptions.SeedMockAuthOptions
	var debug apiserveroptions.DebugOptions
	if opts != nil && opts.SeedMockAuth != nil {
		seed = *opts.SeedMockAuth
	}
	if opts != nil && opts.Debug != nil {
		debug = *opts.Debug
	}
	options := resttransport.RouterOptions{
		DebugCacheGovernance: resttransport.DebugCacheGovernanceOptions{
			AppMode: appMode,
		},
		SeedMockAuth: resttransport.SeedMockAuthOptions{
			Enabled:      seed.Enabled,
			SharedSecret: seed.SharedSecret,
		},
	}

	options.DebugCacheGovernance.Enabled = debug.CacheGovernance.Enabled
	options.DebugCacheGovernance.RequireAdmin = debug.CacheGovernance.RequireAdmin

	return options
}

func (s *apiServer) idpEncryptionSecret() string {
	if s == nil || s.cfg == nil || s.cfg.IDP == nil {
		return ""
	}
	return s.cfg.IDP.EncryptionKey
}
