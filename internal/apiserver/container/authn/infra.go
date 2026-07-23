package authn

import (
	"context"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	authnUow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessionDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	redisInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/cache/redis"
	credentialrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/credential"
	jwksMysql "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/jwks"
	loginidentityrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/loginidentity"
	mysqlAuthnUow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
	jwtinfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/token/jwt"
	"github.com/FangcunMount/iam/v2/internal/apiserver/infra/token/keyset"
	wechatInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/wechat"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
	"github.com/FangcunMount/iam/v2/pkg/event"
)

type authnInfrastructureComponents struct {
	db         *gorm.DB
	redis      *redis.Client
	unitOfWork authnUow.UnitOfWork

	credentialRepo     *credentialrepo.Repository
	loginIdentityRepo  authentication.LoginIdentityRepository
	loginIdentityStore *loginidentityrepo.Repository
	challengeRepo      *redisInfra.ChallengeRepository
	otpRedis           *redisInfra.OTPVerifierImpl
	idp                authentication.IdentityProvider
	accessChecker      sessionDomain.SubjectAccessEvaluator

	keyRepo           keyset.Repository
	privateKeyStorage keyset.PrivateKeyStorage
	keyGenerator      keyset.KeyGenerator
	privKeyResolver   keyset.PrivateKeyResolver
	keyManager        *keyset.KeyManager
	keySetBuilder     *keyset.KeySetBuilder
	keyRotation       *keyset.KeyRotation
	jwtGenerator      *jwtinfra.Generator

	tokenStore   *redisInfra.RedisStore
	sessionStore *redisInfra.SessionStore

	userRepo userDomain.Repository

	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault

	eventBus       messaging.EventBus
	eventPublisher event.Publisher
}

func (m *AuthnModule) initializeInfrastructure(
	db *gorm.DB,
	redisClient *redis.Client,
	idpDeps *idp.IDPModule,
	eventBus messaging.EventBus,
	eventPublisher event.Publisher,
	environment genericapiserver.Environment,
	authOptions apiserveroptions.AuthOptions,
	jwksOptions apiserveroptions.JWKSOptions,
) *authnInfrastructureComponents {
	infra := &authnInfrastructureComponents{
		db:             db,
		redis:          redisClient,
		eventBus:       eventBus,
		eventPublisher: eventPublisher,
	}

	infra.unitOfWork = mysqlAuthnUow.NewUnitOfWork(db)
	infra.credentialRepo = credentialrepo.NewRepository(db)
	loginIdentityRepo := loginidentityrepo.NewRepository(db)
	infra.loginIdentityRepo = loginIdentityRepo
	infra.loginIdentityStore = loginIdentityRepo

	otpRedis := redisInfra.NewOTPVerifier(redisClient)
	infra.otpRedis = otpRedis
	m.otpInspectorSource = otpRedis
	infra.challengeRepo = redisInfra.NewChallengeRepository(redisClient)
	m.challengeInspectorSource = infra.challengeRepo

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

	infra.keyRepo = jwksMysql.NewKeyRepository(db)
	configureJWKSStorage(infra, jwksOptions)
	configureKeyServices(infra, environment, authOptions, jwksOptions)

	infra.tokenStore = redisInfra.NewRedisStore(redisClient)
	m.tokenStoreInspectorSource = infra.tokenStore
	infra.sessionStore = redisInfra.NewSessionStore(redisClient)
	m.sessionStoreInspector = infra.sessionStore

	infra.userRepo = mysqluser.NewRepository(db)
	infra.accessChecker = sessionDomain.NewSubjectAccessEvaluator(infra.userRepo, loginIdentityRepo)

	return infra
}

func configureJWKSStorage(infra *authnInfrastructureComponents, jwksOptions apiserveroptions.JWKSOptions) {
	keysDir := jwksOptions.KeysDir
	if strings.TrimSpace(keysDir) == "" {
		log.Warnw("jwks.keys_dir is empty; private keys will be looked up in current working directory", "jwks.keys_dir", keysDir)
	} else {
		log.Infow("JWKS keys directory", "jwks.keys_dir", keysDir)
	}
	infra.privateKeyStorage = keyset.NewPEMPrivateKeyStorage(keysDir)
	infra.keyGenerator = keyset.NewRSAKeyGeneratorWithStorage(infra.privateKeyStorage)
	infra.privKeyResolver = keyset.NewPEMPrivateKeyResolver(keysDir)
}

func configureKeyServices(
	infra *authnInfrastructureComponents,
	environment genericapiserver.Environment,
	authOptions apiserveroptions.AuthOptions,
	jwksOptions apiserveroptions.JWKSOptions,
) {
	infra.keyManager = keyset.NewKeyManager(infra.keyRepo, infra.keyGenerator)
	infra.keySetBuilder = keyset.NewKeySetBuilder(infra.keyRepo)
	infra.keyRotation = keyset.NewKeyRotation(
		infra.keyRepo,
		infra.keyGenerator,
		keyset.DefaultRotationPolicy(),
		log.New(log.NewOptions()),
	)
	autoInitializeJWKS(infra.keyManager, environment, jwksOptions, log.New(log.NewOptions()))
	infra.jwtGenerator = jwtinfra.NewGenerator(
		authOptions.JWTIssuer,
		authOptions.AccessTokenAudience,
		keyset.NewJWTKeySource(infra.keyManager, infra.privKeyResolver),
	)
}

func autoInitializeJWKS(
	keyManager *keyset.KeyManager,
	environment genericapiserver.Environment,
	jwksOptions apiserveroptions.JWKSOptions,
	logger log.Logger,
) {
	if !shouldAutoInitializeJWKS(environment, jwksOptions.AutoInit) {
		return
	}

	ctx := context.Background()
	if _, err := keyManager.GetActiveKey(ctx); err == nil {
		logger.Debugw("active jwks key present, skip auto-init")
		return
	}

	now := time.Now()
	if _, err := keyManager.CreateKey(ctx, "RS256", &now, ptrTime(now.AddDate(1, 0, 0))); err != nil {
		logger.Warnw("failed to auto-create jwks active key", "error", err)
		return
	}
	logger.Infow("auto-created initial jwks active key", "alg", "RS256")
}

func shouldAutoInitializeJWKS(environment genericapiserver.Environment, configured bool) bool {
	return configured || environment == genericapiserver.EnvironmentDevelopment
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
