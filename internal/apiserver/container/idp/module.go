package idp

import (
	wechatCache "github.com/silenceper/wechat/v2/cache"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/wechatapp"
	wechatappDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	infraRedis "github.com/FangcunMount/iam/v2/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v2/internal/apiserver/infra/wechatapi"
	wechatapiPort "github.com/FangcunMount/iam/v2/internal/apiserver/infra/wechatapi/port"
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

// InitializeWithDeps 初始化 IDP 模块。
func (m *IDPModule) InitializeWithDeps(deps IDPModuleDeps) error {
	if err := validateIDPModuleDeps(deps); err != nil {
		return err
	}

	if err := m.initializeInfrastructure(deps.DB, deps.RedisClient, deps.EncryptionKey); err != nil {
		return err
	}

	domainServices, err := m.initializeDomain()
	if err != nil {
		return err
	}

	return m.initializeApplication(domainServices)
}

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

func (m *IDPModule) ApplicationCapabilities() ApplicationCapabilities {
	if m == nil {
		return ApplicationCapabilities{}
	}
	return ApplicationCapabilities{
		WechatAppService:           m.WechatAppService,
		WechatAppCredentialService: m.WechatAppCredentialService,
		WechatAppTokenService:      m.WechatAppTokenService,
		WechatAppRepository:        m.wechatAppRepo,
		SecretVault:                m.secretVault,
	}
}
