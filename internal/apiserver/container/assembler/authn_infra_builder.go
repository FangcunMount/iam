package assembler

import (
	"strings"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	authnUow "github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	sessionDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/apiserver/infra/crypto"
	jwtinfra "github.com/FangcunMount/iam/internal/apiserver/infra/jwt"
	acctrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/account"
	credentialrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/credential"
	jwksMysql "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/jwks"
	mysqlAuthnUow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	redisInfra "github.com/FangcunMount/iam/internal/apiserver/infra/redis"
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

	keyRepo           jwks.Repository
	privateKeyStorage jwks.PrivateKeyStorage
	keyGenerator      jwks.KeyGenerator
	privKeyResolver   jwks.PrivateKeyResolver
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
	infra.privateKeyStorage = crypto.NewPEMPrivateKeyStorage(keysDir)
	infra.keyGenerator = crypto.NewRSAKeyGeneratorWithStorage(infra.privateKeyStorage)
	infra.privKeyResolver = crypto.NewPEMPrivateKeyResolver(keysDir)
}
