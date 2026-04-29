package assembler

import (
	"context"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	authnUow "github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	sessionDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	acctrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/account"
	credentialrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/credential"
	jwksMysql "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/jwks"
	mysqlAuthnUow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	redisInfra "github.com/FangcunMount/iam/internal/apiserver/infra/redis"
	jwtinfra "github.com/FangcunMount/iam/internal/apiserver/infra/token/jwt"
	"github.com/FangcunMount/iam/internal/apiserver/infra/token/keyset"
	wechatInfra "github.com/FangcunMount/iam/internal/apiserver/infra/wechat"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
	"github.com/FangcunMount/iam/pkg/event"
)

type authnInfrastructureComponents struct {
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
	idpDeps *IDPModule,
	eventBus messaging.EventBus,
	eventPublisher event.Publisher,
	appMode string,
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
	infra.accountRepo = acctrepo.NewAccountRepository(db)
	infra.credentialRepo = credentialrepo.NewRepository(db)

	otpRedis := redisInfra.NewOTPVerifier(redisClient)
	infra.otpVerifier = otpRedis
	infra.otpRedis = otpRedis
	m.otpInspectorSource = otpRedis

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
	configureKeyServices(infra, appMode, authOptions, jwksOptions)

	infra.tokenStore = redisInfra.NewRedisStore(redisClient)
	m.tokenStoreInspectorSource = infra.tokenStore
	infra.sessionStore = redisInfra.NewSessionStore(redisClient)
	m.sessionStoreInspector = infra.sessionStore

	infra.userRepo = mysqluser.NewRepository(db)
	infra.accessChecker = sessionDomain.NewSubjectAccessEvaluator(infra.userRepo, infra.accountRepo)

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
	appMode string,
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
	autoInitializeJWKS(infra.keyManager, appMode, jwksOptions, log.New(log.NewOptions()))
	infra.jwtGenerator = jwtinfra.NewGenerator(
		authOptions.JWTIssuer,
		authOptions.AccessTokenAudience,
		keyset.NewJWTKeySource(infra.keyManager, infra.privKeyResolver),
	)
}

func autoInitializeJWKS(
	keyManager *keyset.KeyManager,
	appMode string,
	jwksOptions apiserveroptions.JWKSOptions,
	logger log.Logger,
) {
	if !jwksOptions.AutoInit && appMode != "development" {
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

func ptrTime(t time.Time) *time.Time {
	return &t
}
