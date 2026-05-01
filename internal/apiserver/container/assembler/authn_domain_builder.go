package assembler

import (
	"time"

	sessionDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
)

type authnDomainComponents struct {
	sessionManager sessionDomain.Manager
	accessTTL      time.Duration
	refreshTTL     time.Duration
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
	domain.accessTTL = accessTTL
	domain.refreshTTL = refreshTTL

	domain.sessionManager = sessionDomain.NewManager(infra.sessionStore)
	m.sessionManager = domain.sessionManager

	return domain
}
