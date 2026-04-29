package assembler

import (
	redis "github.com/redis/go-redis/v9"
	wechatCache "github.com/silenceper/wechat/v2/cache"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/internal/apiserver/application/idp/wechatapp"
	wechatappDomain "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
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
func (m *IDPModule) CacheFamilyInspectors() []cachegovernance.FamilyInspector {
	inspectors := make([]cachegovernance.FamilyInspector, 0, 2)
	inspectors = append(inspectors, infraRedis.AccessTokenCacheInspectors(m.accessTokenCache)...)
	inspectors = append(inspectors, infraRedis.WechatSDKCacheInspectors(m.wechatSDKCache)...)
	return inspectors
}
