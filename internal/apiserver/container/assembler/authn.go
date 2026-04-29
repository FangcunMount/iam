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
	registerApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/register"
	sessionApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	sessionDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/infra/crypto"
	redisInfra "github.com/FangcunMount/iam/internal/apiserver/infra/redis"
	authngrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/authn"
	authhandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/handler"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
	"github.com/FangcunMount/iam/pkg/event"
)

// AuthnModule 认证模块
type AuthnModule struct {
	// 应用服务
	AccountService          accountApp.AccountApplicationService
	RegisterService         registerApp.RegisterApplicationService
	LoginService            login.LoginApplicationService
	LoginPreparationService loginprep.LoginPreparationService
	TokenService            token.TokenApplicationService
	SessionService          sessionApp.SessionApplicationService

	// JWKS 应用服务
	KeyManagementApp *jwksApp.KeyManagementAppService
	KeyPublishApp    *jwksApp.KeyPublishAppService
	KeyRotationApp   *jwksApp.KeyRotationAppService

	// HTTP 处理器
	AccountHandler      *authhandler.AccountHandler
	AuthHandler         *authhandler.AuthHandler
	JWKSHandler         *authhandler.JWKSHandler
	SessionAdminHandler *authhandler.SessionAdminHandler

	// gRPC 服务
	GRPCService *authngrpc.Service

	// 调度器
	RotationScheduler interface {
		Start(ctx context.Context) error
		Stop() error
		IsRunning() bool
		TriggerNow(ctx context.Context) error
	}

	tokenStoreInspectorSource *redisInfra.RedisStore
	sessionStoreInspector     *redisInfra.SessionStore
	otpInspectorSource        *redisInfra.OTPVerifierImpl
	keySetBuilder             *jwks.KeySetBuilder
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
	infra := m.initializeInfrastructure(deps.DB, deps.RedisClient, deps.IDPModule, deps.EventBus, deps.EventPublisher, deps.JWKS)

	// 初始化领域层
	domain := m.initializeDomain(infra, deps.AppMode, deps.Auth, deps.JWKS)

	// 初始化应用层
	if err := m.initializeApplication(infra, domain, hasher, deps.SMS); err != nil {
		return err
	}

	// 初始化接口层
	m.initializeInterface()

	// 初始化调度器
	m.initializeSchedulers()

	return nil
}

// Cleanup 清理资源
func (m *AuthnModule) Cleanup(ctx context.Context) error {
	if m.RotationScheduler != nil && m.RotationScheduler.IsRunning() {
		if err := m.RotationScheduler.Stop(); err != nil {
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
	if m.keySetBuilder != nil {
		inspectors = append(inspectors, cachegovernance.NewJWKSPublishSnapshotInspector(m.keySetBuilder))
	}
	return inspectors
}

// SessionManager 返回认证模块创建的会话管理器。
func (m *AuthnModule) SessionManager() sessionDomain.Manager {
	return m.sessionManager
}
