package apiserver

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	_ "github.com/FangcunMount/component-base/pkg/messaging/nsq" // 注册 NSQ Provider
	"github.com/FangcunMount/component-base/pkg/shutdown"
	"github.com/FangcunMount/component-base/pkg/shutdown/shutdownmanagers/posixsignal"
	"github.com/FangcunMount/iam/internal/apiserver/config"
	"github.com/FangcunMount/iam/internal/apiserver/container"
	"github.com/FangcunMount/iam/internal/pkg/grpc"
	genericapiserver "github.com/FangcunMount/iam/internal/pkg/server"
	"github.com/spf13/viper"
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
	// runtime task cancellation
	outboxRelayCancel context.CancelFunc
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

	// 创建  服务器
	genericServer, err := buildGenericServer(cfg)
	if err != nil {
		log.Fatalf("Failed to build generic server: %v", err)
		return nil, err
	}

	// 创建 GRPC 服务器
	grpcServer, err := buildGRPCServer(cfg)
	if err != nil {
		log.Fatalf("Failed to build GRPC server: %v", err)
		return nil, err
	}

	// 创建数据库管理器
	dbManager := NewDatabaseManager(cfg)

	// 创建 API 服务器实例
	server := &apiServer{
		cfg:              cfg,
		gs:               gs,
		genericAPIServer: genericServer,
		dbManager:        dbManager,
		grpcServer:       grpcServer,
	}

	return server, nil
}

// PrepareRun 准备运行 API 服务器（六边形架构版本）
func (s *apiServer) PrepareRun() (preparedAPIServer, error) {
	runtime := s.prepareRuntime()
	resources, err := s.prepareResources(runtime)
	if err != nil {
		return preparedAPIServer{}, err
	}

	containerOut, err := s.prepareContainer(runtime, resources)
	if err != nil {
		return preparedAPIServer{}, err
	}

	s.prepareTransports(containerOut)
	s.startRuntimeTasks()
	log.Infow("hexagonal architecture initialized", "mode", runtime.mode, "degraded_startup_allowed", runtime.degradedAllowed)
	s.registerShutdownCallbacks()

	return preparedAPIServer{s}, nil
}

// registerGRPCServices 注册所有 gRPC 服务到 gRPC 服务器
func (s *apiServer) registerGRPCServices() {
	if s.grpcServer == nil {
		log.Warn("gRPC server is nil, skipping service registration")
		return
	}

	if s.container == nil {
		log.Warn("Container is nil, skipping gRPC service registration")
		return
	}

	// 注册认证模块的 gRPC 服务
	if s.container.AuthnModule != nil && s.container.AuthnModule.GRPCService != nil {
		s.container.AuthnModule.GRPCService.Register(s.grpcServer.Server)
		log.Info("📡 Registered Authn gRPC services (AuthService, JWKSService)")
	}

	// 注册用户模块的 gRPC 服务（包含 Identity 相关服务）
	if s.container.UserModule != nil && s.container.UserModule.GRPCService != nil {
		s.container.UserModule.GRPCService.Register(s.grpcServer.Server)
		log.Info("📡 Registered User gRPC services (IdentityRead, GuardianshipQuery, GuardianshipCommand, IdentityLifecycle)")
	}

	// 注册 IDP 模块的 gRPC 服务
	if s.container.IDPModule != nil && s.container.IDPModule.GRPCService != nil {
		s.container.IDPModule.GRPCService.Register(s.grpcServer.Server)
		log.Info("📡 Registered IDP gRPC services (IDPService)")
	}

	// 注册 Authz PDP gRPC
	if s.container.AuthzModule != nil && s.container.AuthzModule.GRPCService != nil {
		s.container.AuthzModule.GRPCService.Register(s.grpcServer.Server)
		log.Info("📡 Registered Authz gRPC services (AuthorizationService)")
	}

	log.Info("✅ All gRPC services registered successfully")

	// 标记所有服务为 SERVING 状态（健康检查）
	s.grpcServer.MarkAllServicesServing()
}

// loadIDPEncryptionKey 解析 IDP 加密密钥，支持 base64、base64url、hex 或纯 32 字节字符串
func loadIDPEncryptionKey() ([]byte, bool, error) {
	secret := strings.TrimSpace(viper.GetString("idp.encryption-key"))
	if secret == "" {
		return nil, false, nil
	}

	type decoder struct {
		name   string
		decode func(string) ([]byte, error)
	}

	decoders := []decoder{
		{name: "base64", decode: base64.StdEncoding.DecodeString},
		{name: "base64_raw", decode: base64.RawStdEncoding.DecodeString},
		{name: "base64_url", decode: base64.URLEncoding.DecodeString},
		{name: "base64_url_raw", decode: base64.RawURLEncoding.DecodeString},
		{name: "hex", decode: hex.DecodeString},
	}

	for _, d := range decoders {
		if decoded, err := d.decode(secret); err == nil {
			if len(decoded) == 32 {
				return decoded, true, nil
			}
			log.Warnf("IDP encryption key decoded via %s but length was %d bytes, expected 32", d.name, len(decoded))
		}
	}

	// 最后尝试直接使用原始字符串字节序列
	if len(secret) == 32 {
		return []byte(secret), true, nil
	}

	return nil, true, fmt.Errorf("invalid encryption key: unable to decode to 32 bytes")
}

func (s *apiServer) validateCriticalModules(degradedAllowed bool) error {
	if s.container == nil {
		return fmt.Errorf("container is nil")
	}

	missing := make([]string, 0, 4)
	if s.container.IDPModule == nil {
		missing = append(missing, "idp")
	}
	if s.container.AuthnModule == nil {
		missing = append(missing, "authn")
	}
	if s.container.AuthzModule == nil {
		missing = append(missing, "authz")
	}
	if s.container.UserModule == nil {
		missing = append(missing, "user")
	}
	if len(missing) == 0 {
		return nil
	}
	if !degradedAllowed {
		return fmt.Errorf("critical modules unavailable: %s", strings.Join(missing, ", "))
	}

	log.Warnw("degraded startup: critical modules unavailable", "modules", missing)
	return nil
}

func degradedStartupAllowed(cfg *config.Config) bool {
	if cfg == nil || cfg.GenericServerRunOptions == nil || !cfg.GenericServerRunOptions.AllowDegradedStartup {
		return false
	}
	return !isProductionLike(runtimeMode(cfg))
}

func runtimeMode(cfg *config.Config) string {
	if cfg == nil || cfg.GenericServerRunOptions == nil {
		return "release"
	}
	return strings.ToLower(strings.TrimSpace(cfg.GenericServerRunOptions.Mode))
}

func isProductionLike(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "release", "production":
		return true
	default:
		return false
	}
}

func appModeFromServerMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "release", "production":
		return "production"
	case "test":
		return "test"
	default:
		return "development"
	}
}

// Run 运行 API 服务器
func (s preparedAPIServer) Run() error {
	// 启动关闭管理器
	if err := s.gs.Start(); err != nil {
		log.Fatalf("start shutdown manager failed: %s", err.Error())
	}

	// 创建一个 channel 用于接收错误
	errChan := make(chan error, 2)

	// 启动 HTTP 服务器
	go func() {
		if err := s.genericAPIServer.Run(); err != nil {
			log.Errorf("Failed to run HTTP server: %v", err)
			errChan <- err
		}
	}()
	log.Info("🚀 Starting Hexagonal Architecture HTTP REST API server...")

	// 启动 GRPC 服务器
	go func() {
		if err := s.grpcServer.Run(); err != nil {
			log.Errorf("Failed to run GRPC server: %v", err)
			errChan <- err
		}
	}()
	log.Info("🚀 Starting Hexagonal Architecture GRPC server...")

	// 等待任一服务出错
	return <-errChan
}

// buildGenericServer 构建通用服务器
func buildGenericServer(cfg *config.Config) (*genericapiserver.GenericAPIServer, error) {
	// 构建通用配置
	genericConfig, err := buildGenericConfig(cfg)
	if err != nil {
		return nil, err
	}

	// 完成通用配置并创建实例
	genericServer, err := genericConfig.Complete().New()
	if err != nil {
		return nil, err
	}

	return genericServer, nil
}

// buildGenericConfig 构建通用配置
func buildGenericConfig(cfg *config.Config) (genericConfig *genericapiserver.Config, lastErr error) {
	genericConfig = genericapiserver.NewConfig()

	// 应用通用配置
	if lastErr = cfg.GenericServerRunOptions.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	// 应用安全配置
	if lastErr = cfg.SecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	// 应用不安全配置
	if lastErr = cfg.InsecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}
	return
}

// buildGRPCServer 构建 GRPC 服务器
func buildGRPCServer(cfg *config.Config) (*grpc.Server, error) {
	// 创建 GRPC 配置
	grpcConfig := grpc.NewConfig()

	// 应用配置选项
	if err := applyGRPCOptions(cfg, grpcConfig); err != nil {
		return nil, err
	}

	// 完成配置并创建服务器
	return grpcConfig.Complete().New()
}

// applyGRPCOptions 应用 GRPC 选项到配置
func applyGRPCOptions(cfg *config.Config, grpcConfig *grpc.Config) error {
	// 应用基本配置
	grpcConfig.BindAddress = cfg.GRPCOptions.BindAddress
	grpcConfig.BindPort = cfg.GRPCOptions.BindPort
	grpcConfig.HealthzPort = cfg.GRPCOptions.HealthzPort

	// 应用 mTLS 配置
	if cfg.GRPCOptions.MTLS != nil {
		mtlsOpt := cfg.GRPCOptions.MTLS
		grpcConfig.MTLS.Enabled = mtlsOpt.Enabled
		grpcConfig.MTLS.CAFile = mtlsOpt.CAFile
		grpcConfig.MTLS.CADir = mtlsOpt.CADir
		grpcConfig.MTLS.RequireClientCert = mtlsOpt.RequireClientCert
		grpcConfig.MTLS.AllowedCNs = mtlsOpt.AllowedCNs
		grpcConfig.MTLS.AllowedOUs = mtlsOpt.AllowedOUs
		grpcConfig.MTLS.AllowedSANs = mtlsOpt.AllowedSANs
		grpcConfig.MTLS.MinTLSVersion = mtlsOpt.MinTLSVersion
		grpcConfig.MTLS.EnableAutoReload = mtlsOpt.EnableAutoReload
		if mtlsOpt.ReloadInterval > 0 {
			grpcConfig.MTLS.ReloadInterval = mtlsOpt.ReloadInterval
		}
		// 证书文件（优先使用 gRPC 段配置）
		if mtlsOpt.CertFile != "" {
			grpcConfig.TLSCertFile = mtlsOpt.CertFile
		}
		if mtlsOpt.KeyFile != "" {
			grpcConfig.TLSKeyFile = mtlsOpt.KeyFile
		}
		// mTLS 启用时关闭 Insecure
		if mtlsOpt.Enabled {
			grpcConfig.Insecure = false
		}
	}

	// 应用层认证
	if cfg.GRPCOptions.Auth != nil {
		authOpt := cfg.GRPCOptions.Auth
		grpcConfig.Auth.Enabled = authOpt.Enabled
		grpcConfig.Auth.EnableBearer = authOpt.EnableBearer
		grpcConfig.Auth.EnableHMAC = authOpt.EnableHMAC
		grpcConfig.Auth.EnableAPIKey = authOpt.EnableAPIKey
		if authOpt.HMACTimestampValidity > 0 {
			grpcConfig.Auth.HMACTimestampValidity = authOpt.HMACTimestampValidity
		}
		grpcConfig.Auth.RequireIdentityMatch = authOpt.RequireIdentityMatch
	}

	// ACL
	if cfg.GRPCOptions.ACL != nil {
		aclOpt := cfg.GRPCOptions.ACL
		grpcConfig.ACL.Enabled = aclOpt.Enabled
		grpcConfig.ACL.ConfigFile = aclOpt.ConfigFile
		if aclOpt.DefaultPolicy != "" {
			grpcConfig.ACL.DefaultPolicy = aclOpt.DefaultPolicy
		}
	}

	// 审计
	if cfg.GRPCOptions.Audit != nil {
		grpcConfig.Audit.Enabled = cfg.GRPCOptions.Audit.Enabled
	}

	// 应用 TLS 配置
	// 只有在 gRPC 段未提供证书时，才回退到 SecureServing 的证书
	if cfg.SecureServing != nil && grpcConfig.TLSCertFile == "" && grpcConfig.TLSKeyFile == "" {
		grpcConfig.TLSCertFile = cfg.SecureServing.TLS.CertFile
		grpcConfig.TLSKeyFile = cfg.SecureServing.TLS.KeyFile
	}

	// 如果明确禁用 Insecure，则覆盖默认值
	grpcConfig.Insecure = cfg.GRPCOptions.Insecure && !grpcConfig.MTLS.Enabled && grpcConfig.TLSCertFile == "" && grpcConfig.TLSKeyFile == ""

	return nil
}

// createEventBus 创建消息总线（如果配置启用了 NSQ）
func (s *apiServer) createEventBus() (messaging.EventBus, error) {
	// 从 viper 读取 NSQ 配置
	enabled := viper.GetBool("nsq.enabled")
	if !enabled {
		log.Info("📨 NSQ EventBus: disabled")
		return nil, nil
	}

	msgTimeoutSec := viper.GetInt("nsq.msg-timeout")
	if msgTimeoutSec == 0 {
		msgTimeoutSec = 60
	}
	requeueDelaySec := viper.GetInt("nsq.requeue-delay")
	if requeueDelaySec == 0 {
		requeueDelaySec = 5
	}

	// 构建 NSQ 配置（与 genericoptions.NSQOptions.ToMessagingConfig 对齐，避免 ReadTimeout 等为 0 导致 go-nsq 校验失败）
	cfg := &messaging.Config{
		Provider: messaging.ProviderNSQ,
		NSQ: messaging.NSQConfig{
			LookupdAddrs: viper.GetStringSlice("nsq.lookupd-addrs"),
			NSQdAddr:     viper.GetString("nsq.nsqd-addr"),
			MaxAttempts:  uint16(viper.GetInt("nsq.max-attempts")),
			MaxInFlight:  viper.GetInt("nsq.max-in-flight"),
			MsgTimeout:   time.Duration(msgTimeoutSec) * time.Second,
			RequeueDelay: time.Duration(requeueDelaySec) * time.Second,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
	}

	// 设置默认值
	if len(cfg.NSQ.LookupdAddrs) == 0 {
		cfg.NSQ.LookupdAddrs = []string{"127.0.0.1:4161"}
	}
	if cfg.NSQ.NSQdAddr == "" {
		cfg.NSQ.NSQdAddr = "127.0.0.1:4150"
	}
	if cfg.NSQ.MaxAttempts == 0 {
		cfg.NSQ.MaxAttempts = 5
	}
	if cfg.NSQ.MaxInFlight == 0 {
		cfg.NSQ.MaxInFlight = 200
	}

	// 创建 EventBus
	eventBus, err := messaging.NewEventBus(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSQ EventBus: %w", err)
	}

	log.Info("📨 NSQ EventBus: enabled",
		log.Strings("lookupd_addrs", cfg.NSQ.LookupdAddrs),
		log.String("nsqd_addr", cfg.NSQ.NSQdAddr),
	)

	return eventBus, nil
}
