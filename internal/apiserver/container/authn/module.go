package authn

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/log"
	challengeApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin"
	signupApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signup"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/v3/internal/apiserver/application/cachegovernance"
	sessionDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	redisInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/crypto"
)

// AuthnModule 认证模块
type AuthnModule struct {
	// 应用服务
	signupService                signupApp.SignupService
	loginIdentityLinking         linkingApp.Linker
	sessionService               session.ApplicationService
	sessionRevokeApp             session.Revoker
	challengeService             challengeApp.Service
	startWechatOpenAuthorize     *signin.StartWechatOpenAuthorize
	startWechatOpenLinkAuthorize *linkingApp.StartWechatOpenLinkAuthorize
	completeWechatOpenLink       *linkingApp.CompleteWechatOpenLink
	wechatOpenConfig             WechatOpenConfig
	tokenService                 token.TokenApplicationService

	// JWKS 应用服务
	keyManagementApp *jwksApp.KeyManagementAppService
	keyPublishApp    *jwksApp.KeyPublishAppService
	keyLifecycleApp  *jwksApp.KeyLifecycleAppService

	// 调度器
	rotationScheduler KeyRotationScheduler
	rotationOptions   jwksRotationRuntimeOptions

	tokenStoreInspectorSource *redisInfra.RedisStore
	sessionStoreInspector     *redisInfra.SessionStore
	challengeInspectorSource  *redisInfra.ChallengeRepository
	otpInspectorSource        *redisInfra.OTPVerifierImpl
	jwksSnapshotReporter      jwksApp.SnapshotReporter
	sessionCreator            sessionDomain.Creator
	sessionLoader             sessionDomain.Loader
	sessionRevoker            sessionDomain.Revoker
	sessionExtender           sessionDomain.Extender
	sessionRefreshExpirer     sessionDomain.RefreshExpirer
}

// NewAuthnModule 创建认证模块
func NewAuthnModule() *AuthnModule {
	return &AuthnModule{}
}

// InitializeWithDeps initializes the module through typed dependencies.
func (m *AuthnModule) InitializeWithDeps(deps AuthnModuleDeps) error {
	if deps.DB == nil {
		log.Errorf("AuthnModuleDeps.DB must be *gorm.DB")
		return fmt.Errorf("invalid AuthnModuleDeps.DB")
	}
	if deps.RedisClient == nil {
		log.Errorf("AuthnModuleDeps.RedisClient must be *redis.Client")
		return fmt.Errorf("invalid AuthnModuleDeps.RedisClient")
	}

	hasher := deps.PasswordHasher
	if hasher == nil {
		hasher = crypto.NewArgon2Hasher("")
	}

	// 初始化基础设施层
	infra, err := m.initializeInfrastructure(deps.DB, deps.RedisClient, deps.IDPModule, deps.EventPublisher, deps.Environment, deps.Auth, deps.JWKS)
	if err != nil {
		return err
	}

	// 初始化领域层
	domain := m.initializeDomain(infra, deps.Auth)

	// 初始化应用层
	if err := m.initializeApplication(infra, domain, hasher, deps.Auth, deps.WechatOpen, deps.SMS); err != nil {
		return err
	}

	// 初始化调度器
	m.initializeSchedulers(deps.JWKS)

	return nil
}

// Cleanup 清理资源
func (m *AuthnModule) Cleanup(ctx context.Context) error {
	if m.rotationScheduler != nil && m.rotationScheduler.IsRunning() {
		if err := m.rotationScheduler.Stop(); err != nil {
			log.Warnf("Failed to stop rotation scheduler: %v", err)
		}
	}
	return nil
}

// CacheFamilyInspectors 返回认证模块暴露的缓存族状态读取器。
func (m *AuthnModule) CacheFamilyInspectors() []cachegovernance.FamilyInspector {
	inspectors := make([]cachegovernance.FamilyInspector, 0, 8)
	inspectors = append(inspectors, redisInfra.RedisStoreInspectors(m.tokenStoreInspectorSource)...)
	inspectors = append(inspectors, redisInfra.SessionStoreInspectors(m.sessionStoreInspector)...)
	inspectors = append(inspectors, redisInfra.ChallengeRepositoryInspectors(m.challengeInspectorSource)...)
	inspectors = append(inspectors, redisInfra.OTPVerifierInspectors(m.otpInspectorSource)...)
	if m.jwksSnapshotReporter != nil {
		inspectors = append(inspectors, cachegovernance.NewJWKSPublishSnapshotInspector(m.jwksSnapshotReporter))
	}
	return inspectors
}

// SessionRevoker 返回认证模块创建的会话撤销器。
func (m *AuthnModule) SessionRevoker() sessionDomain.Revoker {
	return m.sessionRevoker
}

func (m *AuthnModule) ApplicationCapabilities() ApplicationCapabilities {
	if m == nil {
		return ApplicationCapabilities{}
	}
	return ApplicationCapabilities{
		SignupService:                m.signupService,
		LoginIdentityLinking:         m.loginIdentityLinking,
		SessionService:               m.sessionService,
		SessionRevoker:               m.sessionRevokeApp,
		LoginPhoneOTPSender:          m.challengeService,
		PhoneLinkOTPSender:           m.challengeService,
		StartWechatOpenAuthorize:     m.startWechatOpenAuthorize,
		StartWechatOpenLinkAuthorize: m.startWechatOpenLinkAuthorize,
		CompleteWechatOpenLink:       m.completeWechatOpenLink,
		WechatOpen:                   m.wechatOpenConfig,
		TokenService:                 m.tokenService,
		KeyManagementApp:             m.keyManagementApp,
		KeyPublishApp:                m.keyPublishApp,
		KeyLifecycleApp:              m.keyLifecycleApp,
	}
}

func (m *AuthnModule) RuntimeCapabilities() RuntimeCapabilities {
	if m == nil {
		return RuntimeCapabilities{}
	}
	return RuntimeCapabilities{
		RotationScheduler: m.rotationScheduler,
	}
}
