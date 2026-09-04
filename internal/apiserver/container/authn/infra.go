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
	externalidentity "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/idp"
	admissionDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	userDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	redisInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/cache/redis"
	credentialrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/credential"
	jwksMysql "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/jwks"
	loginidentityrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/loginidentity"
	mysqlAuthnUow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/user"
	jwtinfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/jwt"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/keyset"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
	pkgauth "github.com/FangcunMount/iam/v3/pkg/auth"
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
	externalResolver   externalidentity.Resolver
	admissionPolicy    admissionDomain.Policy

	keyRepo           keyset.Repository
	privateKeyStorage keyset.PrivateKeyStorage
	keyGenerator      keyset.KeyGenerator
	privKeyResolver   keyset.PrivateKeyResolver
	keyManager        *keyset.KeyManager
	keySetBuilder     *keyset.KeySetBuilder
	keyRotation       *keyset.KeyRotation
	signedJWTCodec    *jwtinfra.JWSCompactTokenCodec

	tokenStore   *redisInfra.RedisStore
	sessionStore *redisInfra.SessionStore
	userRepo     userDomain.Repository

	eventPublisher event.Publisher
}

func (m *AuthnModule) initializeInfrastructure(
	db *gorm.DB,
	redisClient *redis.Client,
	idpDeps *idp.IDPModule,
	eventPublisher event.Publisher,
	userStatusReader useraccess.UserStatusReader,
	environment genericapiserver.Environment,
	authOptions apiserveroptions.AuthOptions,
	jwksOptions apiserveroptions.SigningKeyOptions,
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
		infra.externalResolver = idpDeps.ExternalIdentityResolver()
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
	// Signup is a cross-module transaction that creates/repairs the User aggregate.
	// Status reads used by authentication flow through the injected narrow capability below.
	infra.userRepo = mysqluser.NewRepository(db)

	infra.admissionPolicy = admissionDomain.NewPolicy(userStatusReader, loginIdentityRepo)

	return infra, nil
}

func configureJWKSStorage(infra *authnInfrastructureComponents, jwksOptions apiserveroptions.SigningKeyOptions) {
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
	jwksOptions apiserveroptions.SigningKeyOptions,
) error {
	policy := keyset.RotationPolicy{
		RotationInterval:   jwksOptions.Rotation.RotationInterval,
		GracePeriod:        jwksOptions.Rotation.GracePeriod,
		MaxPublishableKeys: jwksOptions.Rotation.MaxPublishableKey,
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
	infra.signedJWTCodec = jwtinfra.NewJWSCompactTokenCodec(
		authOptions.JWTIssuer,
		authOptions.AccessTokenAudience,
		keyset.NewJWSKeySourceAdapter(infra.keyManager, infra.privKeyResolver),
	)
	return nil
}

func ensureJWKSReady(
	infra *authnInfrastructureComponents,
	environment genericapiserver.Environment,
	jwksOptions apiserveroptions.SigningKeyOptions,
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
		if _, _, err := infra.keyRotation.Bootstrap(ctx, pkgauth.TokenProfileAlgorithm); err != nil {
			return fmt.Errorf("bootstrap jwks active key: %w", err)
		}
		logger.Infow("auto-created initial jwks active key", "alg", pkgauth.TokenProfileAlgorithm)
	} else if !activeKeys[0].IsValidAt(time.Now()) {
		if !allowCreate || activeKeys[0].IsNotYetValid(time.Now()) {
			return fmt.Errorf("active jwks key is not currently valid")
		}
		if _, _, err := infra.keyRotation.CreateAndActivate(ctx, pkgauth.TokenProfileAlgorithm, nil, nil); err != nil {
			return fmt.Errorf("replace expired jwks active key: %w", err)
		}
		logger.Infow("replaced expired jwks active key", "alg", pkgauth.TokenProfileAlgorithm)
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
