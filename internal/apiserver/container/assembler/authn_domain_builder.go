package assembler

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	sessionDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	jwtinfra "github.com/FangcunMount/iam/internal/apiserver/infra/token/jwt"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
)

type authnDomainComponents struct {
	sessionManager sessionDomain.Manager
	tokenCodec     *jwtinfra.Generator
	accessTTL      time.Duration
	refreshTTL     time.Duration

	keyManager    *jwks.KeyManager
	keySetBuilder *jwks.KeySetBuilder
	keyRotation   *jwks.KeyRotation
}

func (m *AuthnModule) initializeDomain(
	infra *authnInfrastructureComponents,
	appMode string,
	authOptions apiserveroptions.AuthOptions,
	jwksOptions apiserveroptions.JWKSOptions,
) *authnDomainComponents {
	domain := &authnDomainComponents{}

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

	autoInitializeJWKS(domain.keyManager, appMode, jwksOptions, logger)

	infra.jwtGenerator = jwtinfra.NewGenerator(
		authOptions.JWTIssuer,
		authOptions.AccessTokenAudience,
		domain.keyManager,
		infra.privKeyResolver,
	)
	domain.tokenCodec = infra.jwtGenerator

	accessTTL := authOptions.AccessTokenTTL
	if accessTTL == 0 {
		accessTTL = 15 * time.Minute
	}
	refreshTTL := authOptions.RefreshTokenTTL
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	domain.accessTTL = accessTTL
	domain.refreshTTL = refreshTTL

	domain.sessionManager = sessionDomain.NewManager(infra.sessionStore)
	m.sessionManager = domain.sessionManager

	return domain
}

func autoInitializeJWKS(
	keyManager *jwks.KeyManager,
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
