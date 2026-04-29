package assembler

import (
	"context"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
	wechatCache "github.com/silenceper/wechat/v2/cache"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/internal/apiserver/application/idp/wechatapp"
	wechatappDomain "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	cacheinfra "github.com/FangcunMount/iam/internal/apiserver/infra/cache"
	"github.com/FangcunMount/iam/internal/apiserver/infra/crypto"
	infraMysql "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/wechatapp"
	infraRedis "github.com/FangcunMount/iam/internal/apiserver/infra/redis"
	"github.com/FangcunMount/iam/internal/apiserver/infra/wechatapi"
	wechatapiPort "github.com/FangcunMount/iam/internal/apiserver/infra/wechatapi/port"
	idpGrpc "github.com/FangcunMount/iam/internal/apiserver/interface/idp/grpc"
	"github.com/FangcunMount/iam/internal/apiserver/interface/idp/restful/handler"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// IDPModule IDP 模块（Identity Provider）
// 负责组装 IDP 相关的所有组件
//
// 架构说明：
// - 直接在容器侧管理基础设施组件，无需中间聚合器
// - 遵循六边形架构：Infrastructure -> Domain -> Application -> Interface
//
// 职责：
// - 微信应用管理（HTTP 接口）
// - 提供基础设施服务（供 authn 模块使用）
// - 认证功能由 authn 模块统一提供
type IDPModule struct {
	// 应用服务（对外暴露）
	WechatAppService           wechatapp.WechatAppApplicationService
	WechatAppCredentialService wechatapp.WechatAppCredentialApplicationService
	WechatAppTokenService      wechatapp.WechatAppTokenApplicationService

	// HTTP 处理器（对外暴露）
	WechatAppHandler *handler.WechatAppHandler
	// WechatAuthHandler 已移除 - 认证由 authn 模块统一提供

	// gRPC 服务（对外暴露）
	GRPCService *idpGrpc.Service

	// 基础设施组件（内部管理，供其他模块使用）
	wechatAppRepo       wechatappDomain.Repository
	accessTokenCache    wechatappDomain.AccessTokenCache
	secretVault         wechatappDomain.SecretVault
	wechatAuthProvider  wechatapiPort.AuthProvider
	wechatTokenProvider *wechatapi.TokenProvider
	wechatSDKCache      wechatCache.Cache
}

// NewIDPModule 创建 IDP 模块
func NewIDPModule() *IDPModule {
	return &IDPModule{}
}

type IDPModuleDeps struct {
	DB            *gorm.DB
	RedisClient   *redis.Client
	EncryptionKey []byte
}

// InitializeWithDeps 初始化 IDP 模块。
func (m *IDPModule) InitializeWithDeps(deps IDPModuleDeps) error {
	if err := validateIDPModuleDeps(deps); err != nil {
		return err
	}

	// 初始化基础设施层组件（直接创建）
	if err := m.initializeInfrastructure(deps.DB, deps.RedisClient, deps.EncryptionKey); err != nil {
		return err
	}

	// 初始化领域层
	domainServices, err := m.initializeDomain()
	if err != nil {
		return err
	}

	// 初始化应用层
	if err := m.initializeApplication(domainServices); err != nil {
		return err
	}

	// 初始化接口层
	if err := m.initializeInterface(); err != nil {
		return err
	}

	return nil
}

// validateIDPModuleDeps 验证初始化依赖。
func validateIDPModuleDeps(deps IDPModuleDeps) error {
	if deps.DB == nil {
		log.Warnf("IDP module initialization requires a valid database connection")
		return errors.WithCode(code.ErrModuleInitializationFailed,
			"database connection is nil or invalid")
	}

	if deps.RedisClient == nil {
		log.Warnf("IDP module initialization requires a valid Redis client")
		return errors.WithCode(code.ErrModuleInitializationFailed,
			"redis client is nil or invalid")
	}

	if len(deps.EncryptionKey) != 32 {
		log.Warnf("IDP module initialization requires a 32-byte encryption key")
		return errors.WithCode(code.ErrModuleInitializationFailed,
			"encryption key must be 32 bytes for AES-256")
	}

	return nil
}

// initializeInfrastructure 初始化基础设施层组件
// 直接创建各个基础设施组件，无需中间聚合器（InfrastructureServices）
func (m *IDPModule) initializeInfrastructure(
	db *gorm.DB,
	redisClient *redis.Client,
	encryptionKey []byte,
) error {
	// 创建 MySQL 仓储
	m.wechatAppRepo = infraMysql.NewWechatAppRepository(db)

	// 创建 Redis 缓存
	m.accessTokenCache = infraRedis.NewAccessTokenCache(redisClient)

	// 创建加密服务
	secretVault, err := crypto.NewSecretVault(encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create secret vault: %w", err)
	}
	m.secretVault = secretVault

	// 创建微信 SDK 使用的缓存适配器（使用 Redis 作为后端）
	wechatSDKCache := infraRedis.NewWechatSDKCache(redisClient)
	m.wechatSDKCache = wechatSDKCache
	m.wechatAuthProvider = wechatapi.NewAuthProvider(wechatSDKCache)
	m.wechatTokenProvider = wechatapi.NewTokenProvider(wechatSDKCache)

	return nil
}

// domainServices 领域层服务（内部结构）
type domainServices struct {
	// 微信应用领域服务
	wechatAppCreator  wechatappDomain.Creator
	credentialRotater wechatappDomain.CredentialRotater
	accessTokenCacher wechatappDomain.AccessTokenCacher
	appTokenProvider  wechatappDomain.AppTokenProvider
}

// initializeDomain 初始化领域层
func (m *IDPModule) initializeDomain() (*domainServices, error) {
	// 创建微信应用领域服务
	wechatAppCreator := wechatappDomain.NewCreator(
		m.wechatAppRepo,
	)

	credentialRotater := wechatappDomain.NewCredentialRotater(
		m.secretVault,
		time.Now,
	)

	// 创建应用令牌提供器适配器（连接基础设施层和领域层）
	appTokenProvider := &appTokenProviderAdapter{
		tokenProvider: m.wechatTokenProvider,
		wechatAppRepo: m.wechatAppRepo,
	}

	accessTokenCacher := wechatappDomain.NewAccessTokenCacher(
		m.accessTokenCache,
		appTokenProvider,
	)

	return &domainServices{
		wechatAppCreator:  wechatAppCreator,
		credentialRotater: credentialRotater,
		accessTokenCacher: accessTokenCacher,
		appTokenProvider:  appTokenProvider,
	}, nil
}

// initializeApplication 初始化应用层
func (m *IDPModule) initializeApplication(
	domainServices *domainServices,
) error {
	// 直接创建各个应用服务
	m.WechatAppService = wechatapp.NewWechatAppApplicationService(
		m.wechatAppRepo,
		domainServices.wechatAppCreator,
		domainServices.credentialRotater,
	)

	m.WechatAppCredentialService = wechatapp.NewWechatAppCredentialApplicationService(
		m.wechatAppRepo,
		domainServices.credentialRotater,
	)

	m.WechatAppTokenService = wechatapp.NewWechatAppTokenApplicationService(
		m.wechatAppRepo,
		domainServices.accessTokenCacher,
		domainServices.appTokenProvider,
		m.accessTokenCache,
	)

	return nil
}

// initializeInterface 初始化接口层
func (m *IDPModule) initializeInterface() error {
	// 创建 HTTP 处理器（仅微信应用管理）
	m.WechatAppHandler = handler.NewWechatAppHandler(
		m.WechatAppService,
		m.WechatAppCredentialService,
		m.WechatAppTokenService,
	)

	// 创建 gRPC 服务
	m.GRPCService = idpGrpc.NewService(
		m.WechatAppService,
		m.wechatAppRepo,
		m.secretVault,
	)

	// WechatAuthHandler 已移除 - 认证功能由 authn 模块统一提供
	// authn 模块通过容器依赖注入使用 IDP 模块的基础设施服务

	return nil
}

// ============ 暴露给其他模块的基础设施能力 ============

// Repository 返回微信应用查询能力（供 authn 模块读取配置）
func (m *IDPModule) Repository() wechatappDomain.Repository {
	return m.wechatAppRepo
}

// SecretVault 返回密钥托管能力（供 authn 模块解密 AppSecret）
func (m *IDPModule) SecretVault() wechatappDomain.SecretVault {
	return m.secretVault
}

// WechatAuthProvider 返回微信认证基础能力（调用微信 code2Session 等接口）
func (m *IDPModule) WechatAuthProvider() wechatapiPort.AuthProvider {
	return m.wechatAuthProvider
}

// CacheFamilyInspectors 返回 IDP 模块暴露的缓存族状态读取器。
func (m *IDPModule) CacheFamilyInspectors() []cacheinfra.FamilyInspector {
	inspectors := make([]cacheinfra.FamilyInspector, 0, 2)
	inspectors = append(inspectors, infraRedis.AccessTokenCacheInspectors(m.accessTokenCache)...)
	inspectors = append(inspectors, infraRedis.WechatSDKCacheInspectors(m.wechatSDKCache)...)
	return inspectors
}

// ==================== 适配器 ====================

// appTokenProviderAdapter 应用令牌提供器适配器
// 将基础设施层的 TokenProvider 适配为领域层的 AppTokenProvider 接口
type appTokenProviderAdapter struct {
	tokenProvider *wechatapi.TokenProvider
	wechatAppRepo wechatappDomain.Repository
}

// Fetch 实现 AppTokenProvider 接口
func (a *appTokenProviderAdapter) Fetch(
	ctx context.Context,
	app *wechatappDomain.WechatApp,
) (*wechatappDomain.AppAccessToken, error) {
	// 获取凭据
	if app.Cred == nil || app.Cred.Auth == nil {
		return nil, fmt.Errorf("app credentials not found")
	}

	// 注意：这里需要解密密钥，但适配器不应该直接访问 SecretVault
	// 实际上，AppTokenProvider 应该由应用层调用，而不是在这里直接实现
	// 这个适配器的实现需要重新考虑
	//
	// 正确的做法是：
	// 1. 应用层获取 app 时已经解密了密钥
	// 2. 或者 AppTokenProvider 接口改为接受明文 appID 和 appSecret
	//
	// 这里暂时返回错误，表示需要调整架构
	return nil, fmt.Errorf("not implemented: AppTokenProvider should be called from application layer with decrypted credentials")
}
