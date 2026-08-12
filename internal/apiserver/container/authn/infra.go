package authn

import (
	"context"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/log"
	authnUow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/uow"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessionDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	userDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	idpPort "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	redisInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/cache/redis"
	credentialrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/credential"
	jwksMysql "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/jwks"
	loginidentityrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/loginidentity"
	mysqlAuthnUow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/user"
	jwtinfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/jwt"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/keyset"
	wechatInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/wechat"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
	"github.com/FangcunMount/iam/v3/pkg/event"
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

	eventPublisher event.Publisher
}

func (m *AuthnModule) initializeInfrastructure(
	db *gorm.DB,
	redisClient *redis.Client,
	idpDeps *idp.IDPModule,
	eventPublisher event.Publisher,
	environment genericapiserver.Environment,
	authOptions apiserveroptions.AuthOptions,
	jwksOptions apiserveroptions.JWKSOptions,
) (*authnInfrastructureComponents, error) {
	infra := &authnInfrastructureComponents{
		db:             db,
		redis:          redisClient,
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
	if err := configureKeyServices(infra, environment, authOptions, jwksOptions); err != nil {
		return nil, err
	}

	infra.tokenStore = redisInfra.NewRedisStore(redisClient)
	m.tokenStoreInspectorSource = infra.tokenStore
	infra.sessionStore = redisInfra.NewSessionStore(redisClient)
	m.sessionStoreInspector = infra.sessionStore

	infra.userRepo = mysqluser.NewRepository(db)
	infra.accessChecker = sessionDomain.NewSubjectAccessEvaluator(infra.userRepo, loginIdentityRepo)

	return infra, nil
}

func configureJWKSStorage(infra *authnInfrastructureComponents, jwksOptions apiserveroptions.JWKSOptions) {
	keysDir := jwksOptions.KeysDir
	if strings.TrimSpace(keysDir) == "" {
		log.Warnw("jwks.keys_dir is empty; private keys will be looked up in current working directory", "jwks.keys_dir", keysDir)
	} else {
		log.Infow("JWKS keys directory", "jwks.keys_dir", keysDir)
	}
	infra.privateKeyStorage = keyset.NewPEMPrivateKeyStorage(keysDir)
	infra.keyGenerator = keyset.NewRSAKeyGenerator()
	infra.privKeyResolver = keyset.NewPEMPrivateKeyResolver(keysDir)
}

func configureKeyServices(
	infra *authnInfrastructureComponents,
	environment genericapiserver.Environment,
	authOptions apiserveroptions.AuthOptions,
	jwksOptions apiserveroptions.JWKSOptions,
) error {
	policy := keyset.RotationPolicy{
		RotationInterval: jwksOptions.Rotation.RotationInterval,
		GracePeriod:      jwksOptions.Rotation.GracePeriod,
		MaxKeysInJWKS:    jwksOptions.Rotation.MaxPublishableKey,
	}
	infra.keyManager = keyset.NewKeyManagerWithPolicy(infra.keyRepo, infra.keyGenerator, infra.privateKeyStorage, policy)
	infra.keySetBuilder = keyset.NewKeySetBuilder(infra.keyRepo)
	infra.keyRotation = keyset.NewKeyRotation(
		infra.keyManager,
		policy,
		log.New(log.NewOptions()),
	)
	if err := ensureJWKSReady(infra, environment, jwksOptions, log.New(log.NewOptions())); err != nil {
		return err
	}
	infra.jwtGenerator = jwtinfra.NewGenerator(
		authOptions.JWTIssuer,
		authOptions.AccessTokenAudience,
		keyset.NewJWTKeySource(infra.keyManager, infra.privKeyResolver),
	)
	return nil
}

func ensureJWKSReady(
	infra *authnInfrastructureComponents,
	environment genericapiserver.Environment,
	jwksOptions apiserveroptions.JWKSOptions,
	logger log.Logger,
) error {
	ctx := context.Background()
	activeKeys, err := infra.keyRepo.FindByStatus(ctx, keyset.KeyActive)
	if err != nil {
		return fmt.Errorf("load active jwks keys: %w", err)
	}
	if len(activeKeys) > 1 {
		return fmt.Errorf("expected exactly one active jwks key, found %d", len(activeKeys))
	}
	allowCreate := shouldAutoInitializeJWKS(environment, jwksOptions.AutoInit)
	if len(activeKeys) == 0 {
		if !allowCreate {
			return fmt.Errorf("no active jwks key and automatic initialization is disabled")
		}
		if _, _, err := infra.keyRotation.Bootstrap(ctx, "RS256"); err != nil {
			return fmt.Errorf("bootstrap jwks active key: %w", err)
		}
		logger.Infow("auto-created initial jwks active key", "alg", "RS256")
	} else if !activeKeys[0].IsValidAt(time.Now()) {
		if !allowCreate || activeKeys[0].IsNotYetValid(time.Now()) {
			return fmt.Errorf("active jwks key is not currently valid")
		}
		if _, _, err := infra.keyRotation.CreateAndActivate(ctx, "RS256", nil, nil); err != nil {
			return fmt.Errorf("replace expired jwks active key: %w", err)
		}
		logger.Infow("replaced expired jwks active key", "alg", "RS256")
	}
	if _, err := infra.keyManager.ValidateActiveKey(ctx, infra.privKeyResolver); err != nil {
		return fmt.Errorf("validate active jwks key material: %w", err)
	}
	if err := infra.keySetBuilder.RefreshCache(ctx); err != nil {
		return fmt.Errorf("initialize jwks publish snapshot: %w", err)
	}
	if err := infra.keyRotation.RefreshStateMetrics(ctx); err != nil {
		logger.Warnw("initialize jwks state metrics failed")
	}
	logger.Debugw("active jwks key and private material validated")
	return nil
}

func shouldAutoInitializeJWKS(environment genericapiserver.Environment, configured bool) bool {
	return configured || environment == genericapiserver.EnvironmentDevelopment
}
