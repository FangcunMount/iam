package grpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"gopkg.in/yaml.v3"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	"github.com/FangcunMount/component-base/pkg/grpc/mtls"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v2/internal/pkg/middleware"
)

// Server GRPC 服务器结构体
type Server struct {
	*grpc.Server
	config        *Config
	services      []Service
	secure        bool
	mtlsEnabled   bool
	mtlsCreds     *mtls.ServerCredentials  // mTLS 凭证，用于证书自动重载
	acl           *interceptors.ServiceACL // 服务级 ACL
	healthServer  *health.Server           // 健康检查服务器
	healthzServer *http.Server             // 独立的 HTTP 健康检查服务器
}

// Service GRPC 服务接口
type Service interface {
	RegisterService(*grpc.Server)
}

// NewServer 创建新的 GRPC 服务器
func NewServer(config *Config) (*Server, error) {
	// 创建 GRPC 服务器选项
	var serverOpts []grpc.ServerOption

	// 安全配置
	secure := false
	mtlsEnabled := false
	var mtlsCreds *mtls.ServerCredentials
	var acl *interceptors.ServiceACL

	// 加载 ACL 配置（需要在构建拦截器之前）
	if config.ACL.Enabled && config.ACL.ConfigFile != "" {
		loadedACL, err := loadACLConfig(config.ACL.ConfigFile, config.ACL.DefaultPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to load ACL config: %w", err)
		}
		acl = loadedACL
		log.Infof("ACL enabled with config file: %s, default policy: %s", config.ACL.ConfigFile, config.ACL.DefaultPolicy)
	}

	// 构建拦截器链
	unaryInterceptors := buildUnaryInterceptors(config, acl)
	streamInterceptors := buildStreamInterceptors(config, acl)

	serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	serverOpts = append(serverOpts, grpc.ChainStreamInterceptor(streamInterceptors...))

	// 添加消息大小限制
	if config.MaxMsgSize > 0 {
		serverOpts = append(serverOpts,
			grpc.MaxRecvMsgSize(config.MaxMsgSize),
			grpc.MaxSendMsgSize(config.MaxMsgSize),
		)
	}

	// 添加连接管理选项
	if config.MaxConnectionAge > 0 {
		serverOpts = append(serverOpts, grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      config.MaxConnectionAge,
			MaxConnectionAgeGrace: config.MaxConnectionAgeGrace,
		}))
	}

	// 优先使用 mTLS，否则回退到单向 TLS
	if !config.Insecure && config.MTLS.Enabled {
		// 使用 mTLS 双向认证
		mtlsCfg := &mtls.Config{
			CertFile:          config.TLSCertFile,
			KeyFile:           config.TLSKeyFile,
			CAFile:            config.MTLS.CAFile,
			CADir:             config.MTLS.CADir, // 支持 CA 证书目录（多级 CA 信任链）
			RequireClientCert: true,
			AllowedCNs:        config.MTLS.AllowedCNs,
			AllowedOUs:        config.MTLS.AllowedOUs,
			AllowedDNSSANs:    config.MTLS.AllowedSANs,
			MinVersion:        parseTLSVersion(config.MTLS.MinTLSVersion),
			EnableAutoReload:  config.MTLS.EnableAutoReload,
			ReloadInterval:    config.MTLS.ReloadInterval,
		}

		creds, err := mtls.NewServerCredentials(mtlsCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create mTLS credentials: %w", err)
		}

		serverOpts = append(serverOpts, creds.GRPCServerOption())
		mtlsCreds = creds
		mtlsEnabled = true
		secure = true

		// 启动证书自动重载
		if config.MTLS.EnableAutoReload {
			creds.StartAutoReload()
		}

		// 记录 CA 配置信息
		caInfo := config.MTLS.CAFile
		if config.MTLS.CADir != "" {
			caInfo = fmt.Sprintf("%s (dir: %s)", caInfo, config.MTLS.CADir)
		}
		log.Infof("mTLS enabled with CA: %s, allowed CNs: %v, allowed OUs: %v",
			caInfo, config.MTLS.AllowedCNs, config.MTLS.AllowedOUs)

	} else if !config.Insecure && config.TLSCertFile != "" && config.TLSKeyFile != "" {
		// 使用单向 TLS
		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %v", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		secure = true
	}

	// 创建 GRPC 服务器
	grpcServer := grpc.NewServer(serverOpts...)

	// 注册健康检查服务
	var healthSrv *health.Server
	if config.EnableHealthCheck {
		healthSrv = health.NewServer()
		healthpb.RegisterHealthServer(grpcServer, healthSrv)
	}

	// 注册反射服务，用于服务发现
	if config.EnableReflection {
		reflection.Register(grpcServer)
	}

	return &Server{
		Server:       grpcServer,
		config:       config,
		services:     make([]Service, 0),
		secure:       secure,
		mtlsEnabled:  mtlsEnabled,
		mtlsCreds:    mtlsCreds,
		acl:          acl,
		healthServer: healthSrv,
	}, nil
}

// buildUnaryInterceptors 构建 Unary 拦截器链
func buildUnaryInterceptors(config *Config, acl *interceptors.ServiceACL) []grpc.UnaryServerInterceptor {
	var chain []grpc.UnaryServerInterceptor

	// 1. 恢复拦截器（最外层，捕获 panic）
	chain = append(chain, RecoveryInterceptor())

	// 2. 请求 ID 拦截器
	chain = append(chain, RequestIDInterceptor())

	// 3. 日志拦截器（使用增强版，支持请求范围 Logger 注入）
	chain = append(chain, middleware.UnaryServerLoggingInterceptor())

	// 4. mTLS 身份提取拦截器（如果启用 mTLS）
	if config.MTLS.Enabled {
		chain = append(chain, interceptors.MTLSInterceptor(
			interceptors.WithMTLSLogger(&logAdapter{}),
		))
		log.Info("mTLS identity interceptor enabled")
	}

	// 5. 凭证验证拦截器（如果启用应用层认证）
	if config.Auth.Enabled {
		extractor := interceptors.NewMetadataCredentialExtractor()
		validator := interceptors.NewCompositeValidator()
		opts := []interceptors.CredentialOption{
			interceptors.WithCredentialLogger(&logAdapter{}),
		}
		if !config.Auth.RequireIdentityMatch {
			opts = append(opts, interceptors.WithoutIdentityMatch())
		}
		chain = append(chain, interceptors.CredentialInterceptor(
			extractor,
			validator,
			opts...,
		))
		log.Infof("Credential interceptor enabled (bearer=%v, hmac=%v, api_key=%v)",
			config.Auth.EnableBearer, config.Auth.EnableHMAC, config.Auth.EnableAPIKey)
	}

	// 6. ACL 拦截器（如果启用）
	if config.ACL.Enabled && acl != nil {
		chain = append(chain, interceptors.ACLInterceptor(
			acl,
			interceptors.WithACLLogger(&logAdapter{}),
		))
		log.Info("ACL interceptor enabled")
	}

	// 7. 审计日志拦截器（如果启用）
	if config.Audit.Enabled {
		chain = append(chain, interceptors.AuditInterceptor(
			interceptors.NewDefaultAuditLogger(&logAdapter{}),
		))
		log.Info("Audit interceptor enabled")
	}

	return chain
}

// buildStreamInterceptors 构建 Stream 拦截器链
func buildStreamInterceptors(config *Config, acl *interceptors.ServiceACL) []grpc.StreamServerInterceptor {
	var chain []grpc.StreamServerInterceptor

	// 流式日志拦截器（使用增强版，支持请求范围 Logger 注入）
	chain = append(chain, middleware.StreamServerLoggingInterceptor())

	// mTLS 流式拦截器
	if config.MTLS.Enabled {
		chain = append(chain, interceptors.MTLSStreamInterceptor(
			interceptors.WithMTLSLogger(&logAdapter{}),
		))
	}

	// 凭证流式拦截器
	if config.Auth.Enabled {
		extractor := interceptors.NewMetadataCredentialExtractor()
		validator := interceptors.NewCompositeValidator()
		chain = append(chain, interceptors.CredentialStreamInterceptor(
			extractor,
			validator,
			interceptors.WithCredentialLogger(&logAdapter{}),
		))
	}

	// ACL 流式拦截器
	if config.ACL.Enabled && acl != nil {
		chain = append(chain, interceptors.ACLStreamInterceptor(
			acl,
			interceptors.WithACLLogger(&logAdapter{}),
		))
	}

	// 审计流式拦截器
	if config.Audit.Enabled {
		chain = append(chain, interceptors.AuditStreamInterceptor(
			interceptors.NewDefaultAuditLogger(&logAdapter{}),
		))
	}

	return chain
}

// logAdapter 适配 component-base 的 log 到 InterceptorLogger 接口
type logAdapter struct{}

func (l *logAdapter) LogInfo(msg string, fields map[string]interface{}) {
	log.Infow(msg, fieldsToArgs(fields)...)
}

func (l *logAdapter) LogError(msg string, fields map[string]interface{}) {
	log.Errorw(msg, fieldsToArgs(fields)...)
}

func fieldsToArgs(fields map[string]interface{}) []interface{} {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return args
}

// loadACLConfig 加载 ACL 配置文件
func loadACLConfig(configFile, defaultPolicy string) (*interceptors.ServiceACL, error) {
	// 从 YAML 文件加载 ACL 配置
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read ACL config file: %w", err)
	}

	var aclConfig interceptors.ACLConfig
	if err := yaml.Unmarshal(data, &aclConfig); err != nil {
		return nil, fmt.Errorf("failed to parse ACL config: %w", err)
	}

	// 如果文件中未指定默认策略，使用参数中的
	if aclConfig.DefaultPolicy == "" {
		aclConfig.DefaultPolicy = defaultPolicy
	}

	return interceptors.NewServiceACL(&aclConfig), nil
}

// parseTLSVersion 解析 TLS 版本字符串
func parseTLSVersion(version string) uint16 {
	switch version {
	case "1.0":
		return 0x0301 // tls.VersionTLS10
	case "1.1":
		return 0x0302 // tls.VersionTLS11
	case "1.2":
		return 0x0303 // tls.VersionTLS12
	case "1.3":
		return 0x0304 // tls.VersionTLS13
	default:
		return 0x0303 // 默认 TLS 1.2
	}
}

// RegisterService 注册 GRPC 服务
func (s *Server) RegisterService(service Service) {
	service.RegisterService(s.Server)
	s.services = append(s.services, service)
}

// SetServingStatus 设置指定服务的健康状态
// service: 服务名称（如 "iam.authn.v2.AuthService"），空字符串表示整体服务
// status: 健康状态（healthpb.HealthCheckResponse_SERVING, NOT_SERVING, UNKNOWN, SERVICE_UNKNOWN）
func (s *Server) SetServingStatus(service string, status healthpb.HealthCheckResponse_ServingStatus) {
	if s.healthServer != nil {
		s.healthServer.SetServingStatus(service, status)
		log.Infof("Set gRPC health status for '%s': %s", service, status.String())
	}
}

// MarkAllServicesServing 将所有已注册的服务标记为 SERVING 状态
// 应在所有服务注册完成后调用
func (s *Server) MarkAllServicesServing() {
	if s.healthServer == nil {
		return
	}

	// 设置整体服务状态为 SERVING
	s.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// 遍历所有已注册的 gRPC 服务，为每个服务设置健康状态
	// 这样客户端可以按服务名称探测健康状态
	serviceInfo := s.Server.GetServiceInfo()
	serviceNames := make([]string, 0, len(serviceInfo))
	for serviceName := range serviceInfo {
		// 跳过内部健康检查服务本身
		if serviceName == "grpc.health.v1.Health" {
			continue
		}
		s.healthServer.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)
		serviceNames = append(serviceNames, serviceName)
	}

	if len(serviceNames) > 0 {
		log.Infof("Marked %d gRPC services as SERVING: %v", len(serviceNames), serviceNames)
	} else {
		log.Info("Marked all gRPC services as SERVING (no specific services registered)")
	}
}

// startHealthzServer 启动独立的 HTTP 健康检查服务器
// 这个 HTTP 端点不需要 mTLS 认证，方便 K8s 和负载均衡器探测
func (s *Server) startHealthzServer() error {
	if !s.config.EnableHealthCheck || s.healthServer == nil {
		return nil
	}

	if s.config.HealthzPort == 0 {
		log.Info("Healthz port is 0, skipping HTTP health check server")
		return nil
	}

	address := fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.HealthzPort)

	// 创建 HTTP 处理器
	mux := http.NewServeMux()

	// /healthz - 简单的健康检查端点
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if s.healthServer == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Health check not enabled"))
			return
		}

		// 检查整体服务状态
		resp, err := s.healthServer.Check(r.Context(), &healthpb.HealthCheckRequest{})
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(fmt.Sprintf("Health check failed: %v", err)))
			return
		}

		if resp.Status == healthpb.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(fmt.Sprintf("Status: %s", resp.Status)))
		}
	})

	// /livez - 存活探针
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// /readyz - 就绪探针
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.healthServer == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Health check not enabled"))
			return
		}

		resp, err := s.healthServer.Check(r.Context(), &healthpb.HealthCheckRequest{})
		if err != nil || resp.Status != healthpb.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("NOT_READY"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("READY"))
	})

	s.healthzServer = &http.Server{
		Addr:    address,
		Handler: mux,
	}

	// 在后台启动 HTTP 服务器
	go func() {
		log.Infof("Starting HTTP health check server on http://%s", address)
		if err := s.healthzServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("HTTP health check server error: %v", err)
		}
	}()

	return nil
}

// Run 启动 GRPC 服务器
func (s *Server) Run() error {
	// 启动独立的 HTTP 健康检查服务器
	if err := s.startHealthzServer(); err != nil {
		return fmt.Errorf("failed to start healthz server: %v", err)
	}

	address := fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.BindPort)

	// 创建 TCP 监听器
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", address, err)
	}

	// 打印服务器信息
	scheme := "http"
	if s.secure {
		scheme = "https"
	}
	log.Infof("Starting GRPC Server on %s://%s (max message size: %d)", scheme, address, s.config.MaxMsgSize)

	// 启动服务器
	return s.Serve(lis)
}

// RunWithContext 使用上下文启动 GRPC 服务器
func (s *Server) RunWithContext(ctx context.Context) error {
	errCh := make(chan error)
	go func() {
		errCh <- s.Run()
	}()

	select {
	case <-ctx.Done():
		s.Close()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// Close 优雅关闭 GRPC 服务器
func (s *Server) Close() {
	// 标记所有服务为 NOT_SERVING 状态，拒绝新的健康检查
	if s.healthServer != nil {
		// 设置整体服务状态为 NOT_SERVING
		s.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

		// 遍历所有已注册的 gRPC 服务，标记为 NOT_SERVING
		serviceInfo := s.Server.GetServiceInfo()
		for serviceName := range serviceInfo {
			if serviceName == "grpc.health.v1.Health" {
				continue
			}
			s.healthServer.SetServingStatus(serviceName, healthpb.HealthCheckResponse_NOT_SERVING)
		}

		log.Info("Marked all gRPC services as NOT_SERVING")
	}

	// 关闭 HTTP 健康检查服务器
	if s.healthzServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.healthzServer.Shutdown(ctx); err != nil {
			log.Errorf("Failed to shutdown healthz server: %v", err)
		} else {
			log.Info("HTTP health check server stopped")
		}
	}

	const timeout = 5 * time.Second
	ch := make(chan struct{})

	go func() {
		// 优雅停止
		s.GracefulStop()
		close(ch)
	}()

	// 等待优雅停止或超时
	select {
	case <-ch:
		log.Info("GRPC server stopped gracefully")
	case <-time.After(timeout):
		log.Info("GRPC server forced to stop after timeout")
		s.Stop()
	}

	// 停止 mTLS 证书自动重载
	if s.mtlsCreds != nil {
		s.mtlsCreds.Stop()
	}
}

// IsMTLSEnabled 返回是否启用了 mTLS
func (s *Server) IsMTLSEnabled() bool {
	return s.mtlsEnabled
}

// Address 返回服务器地址
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.BindPort)
}

// Config 返回服务器配置
func (s *Server) Config() *Config {
	return s.config
}
