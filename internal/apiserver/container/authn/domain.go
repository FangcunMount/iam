package authn

import (
	"time"

	sessionDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
)

type authnDomainComponents struct {
	sessionCreator        sessionDomain.Creator
	sessionLoader         sessionDomain.Loader
	sessionRevoker        sessionDomain.Revoker
	sessionExtender       sessionDomain.Extender
	sessionRefreshExpirer sessionDomain.RefreshExpirer
	accessTTL             time.Duration
}

func (m *AuthnModule) initializeDomain(
	infra *authnInfrastructureComponents,
	authOptions apiserveroptions.AuthOptions,
) *authnDomainComponents {
	domain := &authnDomainComponents{}

	accessTTL := authOptions.AccessTokenTTL
	if accessTTL == 0 {
		accessTTL = 15 * time.Minute
	}
	refreshTTL := authOptions.RefreshTokenTTL
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	sessionMaxTTL := authOptions.SessionMaxTTL
	if sessionMaxTTL == 0 {
		sessionMaxTTL = 24 * time.Hour
	}
	domain.accessTTL = accessTTL

	lifetime := sessionDomain.NewLifetimePolicy(refreshTTL, sessionMaxTTL)
	domain.sessionCreator = sessionDomain.NewCreator(infra.sessionStore, lifetime)
	domain.sessionLoader = sessionDomain.NewLoader(infra.sessionStore, lifetime)
	domain.sessionRevoker = sessionDomain.NewRevoker(infra.sessionStore)
	domain.sessionExtender = sessionDomain.NewExtender(infra.sessionStore, lifetime)
	domain.sessionRefreshExpirer = sessionDomain.NewRefreshExpirer(lifetime)

	m.sessionCreator = domain.sessionCreator
	m.sessionLoader = domain.sessionLoader
	m.sessionRevoker = domain.sessionRevoker
	m.sessionExtender = domain.sessionExtender
	m.sessionRefreshExpirer = domain.sessionRefreshExpirer

	return domain
}
