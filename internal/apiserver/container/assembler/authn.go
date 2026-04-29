package assembler

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	accountApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	jwksApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/login"
	loginprep "github.com/FangcunMount/iam/internal/apiserver/application/authn/loginprep"
	onboardingApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	sessionApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	sessionDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/infra/crypto"
	redisInfra "github.com/FangcunMount/iam/internal/apiserver/infra/redis"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
	"github.com/FangcunMount/iam/pkg/event"
)

// AuthnModule 认证模块
type AuthnModule struct {
	// 应用服务
	accountService          accountApp.AccountApplicationService
	accountOnboarder        onboardingApp.AccountOnboarder
	loginService            login.LoginApplicationService
	loginPreparationService loginprep.LoginPreparationService
	tokenService            token.TokenApplicationService
	sessionService          sessionApp.SessionApplicationService

	// JWKS 应用服务
	keyManagementApp *jwksApp.KeyManagementAppService
	keyPublishApp    *jwksApp.KeyPublishAppService
	keyRotationApp   *jwksApp.KeyRotationAppService

	// 调度器
	rotationScheduler KeyRotationScheduler

	tokenStoreInspectorSource *redisInfra.RedisStore
	sessionStoreInspector     *redisInfra.SessionStore
	otpInspectorSource        *redisInfra.OTPVerifierImpl
	jwksSnapshotReporter      jwksApp.SnapshotReporter
	sessionManager            sessionDomain.Manager
}

// NewAuthnModule 创建认证模块
func NewAuthnModule() *AuthnModule {
	return &AuthnModule{}
}

// AuthnModuleDeps contains the runtime dependencies required to assemble the
// authentication module.
type AuthnModuleDeps struct {
	DB             *gorm.DB
	RedisClient    *redis.Client
	PasswordHasher authentication.PasswordHasher
	IDPModule      *IDPModule
	EventBus       messaging.EventBus
	EventPublisher event.Publisher
	AppMode        string
	Auth           apiserveroptions.AuthOptions
	JWKS           apiserveroptions.JWKSOptions
	IDPOptions     apiserveroptions.IDPOptions
	SMS            apiserveroptions.SMSOptions
}

// InitializeWithDeps initializes the module through typed dependencies.
func (m *AuthnModule) InitializeWithDeps(deps AuthnModuleDeps) error {
	if deps.DB == nil {
		log.Errorf("params[0] must be *gorm.DB")
		return fmt.Errorf("invalid db parameter")
	}
	if deps.RedisClient == nil {
		log.Errorf("params[1] must be *redis.Client")
		return fmt.Errorf("invalid redis parameter")
	}

	hasher := deps.PasswordHasher
	if hasher == nil {
		hasher = crypto.NewArgon2Hasher("")
	}

	// 初始化基础设施层
	infra := m.initializeInfrastructure(deps.DB, deps.RedisClient, deps.IDPModule, deps.EventBus, deps.EventPublisher, deps.AppMode, deps.Auth, deps.JWKS)

	// 初始化领域层
	domain := m.initializeDomain(infra, deps.Auth)

	// 初始化应用层
	if err := m.initializeApplication(infra, domain, hasher, deps.IDPOptions, deps.SMS); err != nil {
		return err
	}

	// 初始化调度器
	m.initializeSchedulers()

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
	inspectors = append(inspectors, redisInfra.OTPVerifierInspectors(m.otpInspectorSource)...)
	if m.jwksSnapshotReporter != nil {
		inspectors = append(inspectors, cachegovernance.NewJWKSPublishSnapshotInspector(m.jwksSnapshotReporter))
	}
	return inspectors
}

// SessionManager 返回认证模块创建的会话管理器。
func (m *AuthnModule) SessionManager() sessionDomain.Manager {
	return m.sessionManager
}

func (m *AuthnModule) ApplicationCapabilities() AuthnApplicationCapabilities {
	if m == nil {
		return AuthnApplicationCapabilities{}
	}
	return AuthnApplicationCapabilities{
		AccountService:          m.accountService,
		AccountOnboarder:        m.accountOnboarder,
		LoginService:            m.loginService,
		LoginPreparationService: m.loginPreparationService,
		TokenService:            m.tokenService,
		SessionService:          m.sessionService,
		KeyManagementApp:        m.keyManagementApp,
		KeyPublishApp:           m.keyPublishApp,
		KeyRotationApp:          m.keyRotationApp,
	}
}

func (m *AuthnModule) RuntimeCapabilities() AuthnRuntimeCapabilities {
	if m == nil {
		return AuthnRuntimeCapabilities{}
	}
	return AuthnRuntimeCapabilities{
		RotationScheduler: m.rotationScheduler,
	}
}
