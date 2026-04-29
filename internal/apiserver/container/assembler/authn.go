package assembler

import (
	"context"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
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
	authnUow "github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	sessionDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	tokenDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	authenticationInfra "github.com/FangcunMount/iam/internal/apiserver/infra/authentication"
	cacheinfra "github.com/FangcunMount/iam/internal/apiserver/infra/cache"
	"github.com/FangcunMount/iam/internal/apiserver/infra/crypto"
	jwtinfra "github.com/FangcunMount/iam/internal/apiserver/infra/jwt"
	acctrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/account"
	credentialrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/credential"
	jwksMysql "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/jwks"
	mysqlAuthnUow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	redisInfra "github.com/FangcunMount/iam/internal/apiserver/infra/redis"
	schedulerInfra "github.com/FangcunMount/iam/internal/apiserver/infra/scheduler"
	smsInfra "github.com/FangcunMount/iam/internal/apiserver/infra/sms"
	wechatInfra "github.com/FangcunMount/iam/internal/apiserver/infra/wechat"
	authngrpc "github.com/FangcunMount/iam/internal/apiserver/interface/authn/grpc"
	authhandler "github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful/handler"
	"github.com/FangcunMount/iam/internal/pkg/event"
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

// Initialize 初始化模块
// params[0]: *gorm.DB
// params[1]: *redis.Client
// 可选参数：
//   - authentication.PasswordHasher 自定义密码哈希器
//   - *IDPModule              注入 IDP 模块提供的基础设施能力
//   - messaging.EventBus      可选；sms.provider=mq 时用于发布登录 OTP 短信任务
//   - event.Publisher         可选；优先用于 catalog-backed best-effort 事件发布
func (m *AuthnModule) Initialize(params ...interface{}) error {
	if len(params) < 2 {
		log.Errorf("AuthnModule.Initialize requires at least 2 parameters: db, redisClient")
		return fmt.Errorf("requires at least 2 parameters")
	}

	db, ok := params[0].(*gorm.DB)
	if !ok || db == nil {
		log.Errorf("params[0] must be *gorm.DB")
		return fmt.Errorf("invalid db parameter")
	}

	redisClient, ok := params[1].(*redis.Client)
	if !ok || redisClient == nil {
		log.Errorf("params[1] must be *redis.Client")
		return fmt.Errorf("invalid redis parameter")
	}

	// 获取可选依赖
	var (
		hasher         authentication.PasswordHasher
		idpDeps        *IDPModule
		eventBus       messaging.EventBus
		eventPublisher event.Publisher
	)
	for _, opt := range params[2:] {
		switch v := opt.(type) {
		case authentication.PasswordHasher:
			hasher = v
		case *IDPModule:
			idpDeps = v
		case messaging.EventBus:
			eventBus = v
		case event.Publisher:
			eventPublisher = v
		}
	}
	if hasher == nil {
		hasher = crypto.NewArgon2Hasher("")
	}

	// 初始化基础设施层
	infra := m.initializeInfrastructure(db, redisClient, idpDeps, eventBus, eventPublisher)

	// 初始化领域层
	domain := m.initializeDomain(infra)

	// 初始化应用层
	if err := m.initializeApplication(infra, domain, hasher); err != nil {
		return err
	}

	// 初始化接口层
	m.initializeInterface()

	// 初始化调度器
	m.initializeSchedulers()

	return nil
}

// infrastructureComponents 基础设施层组件
type infrastructureComponents struct {
	db         *gorm.DB
	redis      *redis.Client
	unitOfWork authnUow.UnitOfWork

	accountRepo    *acctrepo.AccountRepository
	credentialRepo authentication.CredentialRepository
	otpVerifier    authentication.OTPVerifier
	otpRedis       *redisInfra.OTPVerifierImpl
	idp            authentication.IdentityProvider
	tokenVerifier  authentication.TokenVerifier
	accessChecker  sessionDomain.SubjectAccessEvaluator

	// JWKS 相关
	keyRepo           jwks.Repository
	privateKeyStorage jwks.PrivateKeyStorage
	keyGenerator      jwks.KeyGenerator
	privKeyResolver   jwks.PrivateKeyResolver
	jwtGenerator      *jwtinfra.Generator

	// Token 存储
	tokenStore   *redisInfra.RedisStore
	sessionStore *redisInfra.SessionStore

	// User 仓储
	userRepo userDomain.Repository

	// IDP 基础设施
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault

	// 消息总线（可选，登录 OTP 走 MQ 时需要）
	eventBus       messaging.EventBus
	eventPublisher event.Publisher
}

// initializeInfrastructure 初始化基础设施层
func (m *AuthnModule) initializeInfrastructure(db *gorm.DB, redisClient *redis.Client, idpDeps *IDPModule, eventBus messaging.EventBus, eventPublisher event.Publisher) *infrastructureComponents {
	infra := &infrastructureComponents{
		db:             db,
		redis:          redisClient,
		eventBus:       eventBus,
		eventPublisher: eventPublisher,
	}

	// UnitOfWork
	infra.unitOfWork = mysqlAuthnUow.NewUnitOfWork(db)
	infra.accountRepo = acctrepo.NewAccountRepository(db)
	infra.credentialRepo = credentialrepo.NewRepository(db)

	// OTP：验证 / 写入 / 发送频控共用同一 Redis 实现
	otpRedis := redisInfra.NewOTPVerifier(redisClient)
	infra.otpVerifier = otpRedis
	infra.otpRedis = otpRedis
	m.otpInspectorSource = otpRedis

	// 身份提供商 (微信)
	// 优先使用 IDP 模块提供的基础设施能力
	if idpDeps != nil {
		infra.wechatAppQuerier = idpDeps.Repository()
		infra.secretVault = idpDeps.SecretVault()
		if provider := idpDeps.WechatAuthProvider(); provider != nil {
			infra.idp = wechatInfra.NewIdentityProvider(provider, nil)
		}
	}
	if infra.idp == nil {
		infra.idp = wechatInfra.NewIdentityProvider(nil, nil)
	}

	// JWKS 仓储
	infra.keyRepo = jwksMysql.NewKeyRepository(db)

	// JWKS 基础设施
	keysDir := viper.GetString("jwks.keys_dir")
	// 打印 keys_dir 以便启动时诊断（如果为空，会提示警告）
	if strings.TrimSpace(keysDir) == "" {
		log.Warnw("jwks.keys_dir is empty; private keys will be looked up in current working directory", "jwks.keys_dir", keysDir)
	} else {
		log.Infow("JWKS keys directory", "jwks.keys_dir", keysDir)
	}
	infra.privateKeyStorage = crypto.NewPEMPrivateKeyStorage(keysDir)
	infra.keyGenerator = crypto.NewRSAKeyGeneratorWithStorage(infra.privateKeyStorage)
	infra.privKeyResolver = crypto.NewPEMPrivateKeyResolver(keysDir)

	// Token Store
	infra.tokenStore = redisInfra.NewRedisStore(redisClient)
	m.tokenStoreInspectorSource = infra.tokenStore
	infra.sessionStore = redisInfra.NewSessionStore(redisClient)
	m.sessionStoreInspector = infra.sessionStore

	// User 仓储（跨模块依赖）
	infra.userRepo = mysqluser.NewRepository(db)
	infra.accessChecker = sessionDomain.NewSubjectAccessEvaluator(infra.userRepo, infra.accountRepo)

	return infra
}

// domainComponents 领域层组件
type domainComponents struct {
	// Token 服务
	tokenIssuer    *tokenDomain.TokenIssuer
	tokenRefresher *tokenDomain.TokenRefresher
	tokenVerifyer  *tokenDomain.TokenVerifyer
	sessionManager sessionDomain.Manager

	// JWKS 服务
	keyManager    *jwks.KeyManager
	keySetBuilder *jwks.KeySetBuilder
	keyRotation   *jwks.KeyRotation
}

// initializeDomain 初始化领域层
func (m *AuthnModule) initializeDomain(infra *infrastructureComponents) *domainComponents {
	domain := &domainComponents{}

	// JWKS 领域服务
	domain.keyManager = jwks.NewKeyManager(infra.keyRepo, infra.keyGenerator)
	domain.keySetBuilder = jwks.NewKeySetBuilder(infra.keyRepo)
	m.keySetBuilder = domain.keySetBuilder

	rotationPolicy := jwks.DefaultRotationPolicy()
	logger := log.New(log.NewOptions())
	domain.keyRotation = jwks.NewKeyRotation(
		infra.keyRepo,
		infra.keyGenerator,
		rotationPolicy,
		logger,
	)

	// Auto-initialize JWKS: ensure there's at least one active key in development
	// or when jwks.auto_init is explicitly enabled.
	if viper.GetBool("jwks.auto_init") || viper.GetString("app.mode") == "development" {
		ctx := context.Background()
		if _, err := domain.keyManager.GetActiveKey(ctx); err != nil {
			// 没有 active key，尝试创建一个
			now := time.Now()
			if _, err := domain.keyManager.CreateKey(ctx, "RS256", &now, ptrTime(now.AddDate(1, 0, 0))); err != nil {
				logger.Warnw("failed to auto-create jwks active key", "error", err)
			} else {
				logger.Infow("auto-created initial jwks active key", "alg", "RS256")
			}
		} else {
			logger.Debugw("active jwks key present, skip auto-init")
		}
	}

	// JWT Generator（依赖 JWKS）
	infra.jwtGenerator = jwtinfra.NewGenerator(
		viper.GetString("auth.jwt_issuer"),
		viper.GetStringSlice("auth.access_token_audience"),
		domain.keyManager,
		infra.privKeyResolver,
	)

	// Token 领域服务
	accessTTL := viper.GetDuration("auth.access_token_ttl")
	if accessTTL == 0 {
		accessTTL = 15 * 60 * 1000000000 // 15分钟（纳秒）
	}
	refreshTTL := viper.GetDuration("auth.refresh_token_ttl")
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * 60 * 60 * 1000000000 // 7天（纳秒）
	}

	domain.sessionManager = sessionDomain.NewManager(infra.sessionStore)
	m.sessionManager = domain.sessionManager
	domain.tokenIssuer = tokenDomain.NewTokenIssuer(infra.jwtGenerator, infra.tokenStore, domain.sessionManager, accessTTL, refreshTTL)
	domain.tokenRefresher = tokenDomain.NewTokenRefresher(infra.jwtGenerator, infra.tokenStore, domain.sessionManager, infra.accessChecker, accessTTL, refreshTTL)
	domain.tokenVerifyer = tokenDomain.NewTokenVerifyer(infra.jwtGenerator, infra.tokenStore, domain.sessionManager, infra.accessChecker)

	// 创建 TokenVerifier 适配器供 authentication 模块使用
	infra.tokenVerifier = authenticationInfra.NewTokenVerifierAdapter(domain.tokenVerifyer)

	return domain
}

// ptrTime 返回时间指针（本文件局部辅助函数）
func ptrTime(t time.Time) *time.Time {
	return &t
}

// initializeApplication 初始化应用层
func (m *AuthnModule) initializeApplication(
	infra *infrastructureComponents,
	domain *domainComponents,
	hasher authentication.PasswordHasher,
) error {
	// 账户应用服务
	m.AccountService = accountApp.NewAccountApplicationService(infra.unitOfWork, domain.sessionManager)

	// 注册服务
	m.RegisterService = registerApp.NewRegisterApplicationService(
		infra.unitOfWork,
		hasher,
		infra.idp,
		infra.userRepo,
		infra.wechatAppQuerier,
		infra.secretVault,
	)

	smsProvider := strings.ToLower(strings.TrimSpace(viper.GetString("sms.provider")))
	if smsProvider == "" {
		smsProvider = "log"
	}
	var smsSender authentication.SMSSender
	switch smsProvider {
	case "log":
		smsSender = smsInfra.LogSender{}
	case "mq":
		if infra.eventPublisher != nil {
			smsSender = smsInfra.NewMQLoginOTPSenderWithPublisher(infra.eventPublisher)
			break
		}
		if infra.eventBus == nil {
			return fmt.Errorf("sms.provider=mq requires EventBus or event publisher (enable nsq.enabled and ensure EventBus is created)")
		}
		topic := strings.TrimSpace(viper.GetString("sms.mq.topic"))
		smsSender = smsInfra.NewMQLoginOTPSender(infra.eventBus, topic)
	default:
		log.Warnw("unknown sms.provider, fallback to log", "sms.provider", smsProvider)
		smsSender = smsInfra.LogSender{}
	}

	phoneOTP := &loginprep.PhoneOTPDeps{
		Store:    infra.otpRedis,
		Gate:     infra.otpRedis,
		SMS:      smsSender,
		TTL:      viper.GetDuration("sms.login_otp_ttl"),
		Cooldown: viper.GetDuration("sms.login_otp_send_cooldown"),
		CodeLen:  viper.GetInt("sms.login_otp_code_length"),
	}

	m.LoginPreparationService = loginprep.NewLoginPreparationService(phoneOTP)

	m.LoginService = login.NewLoginApplicationService(
		domain.tokenIssuer,
		domain.tokenRefresher,
		authentication.NewAuthenticater(
			infra.credentialRepo,
			infra.accountRepo,
			hasher,
			infra.otpVerifier,
			infra.idp,
			infra.tokenVerifier,
		),
		infra.wechatAppQuerier,
		infra.secretVault,
	)

	// Token 服务
	m.TokenService = token.NewTokenApplicationService(
		domain.tokenIssuer,
		domain.tokenRefresher,
		domain.tokenVerifyer,
	)
	m.SessionService = sessionApp.NewSessionApplicationService(domain.sessionManager)

	// JWKS 应用服务
	logger := log.New(log.NewOptions())
	m.KeyManagementApp = jwksApp.NewKeyManagementAppService(domain.keyManager, logger)
	m.KeyPublishApp = jwksApp.NewKeyPublishAppService(domain.keySetBuilder, logger)
	m.KeyRotationApp = jwksApp.NewKeyRotationAppService(domain.keyRotation, logger)

	return nil
}

// initializeInterface 初始化接口层
func (m *AuthnModule) initializeInterface() {
	m.AccountHandler = authhandler.NewAccountHandler(
		m.AccountService,
		m.RegisterService,
	)

	m.AuthHandler = authhandler.NewAuthHandler(
		m.LoginService,
		m.TokenService,
		m.LoginPreparationService,
	)

	m.JWKSHandler = authhandler.NewJWKSHandler(
		m.KeyManagementApp,
		m.KeyPublishApp,
	)
	m.SessionAdminHandler = authhandler.NewSessionAdminHandler(m.SessionService)

	m.GRPCService = authngrpc.NewService(
		m.TokenService,
		m.RegisterService,
		m.KeyPublishApp,
	)
}

// initializeSchedulers 初始化调度器
func (m *AuthnModule) initializeSchedulers() {
	logger := log.New(log.NewOptions())
	cronSpec := "0 2 * * *" // 每天凌晨2点

	m.RotationScheduler = schedulerInfra.NewKeyRotationCronScheduler(
		m.KeyRotationApp,
		cronSpec,
		logger,
	)
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
func (m *AuthnModule) CacheFamilyInspectors() []cacheinfra.FamilyInspector {
	inspectors := make([]cacheinfra.FamilyInspector, 0, 8)
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
