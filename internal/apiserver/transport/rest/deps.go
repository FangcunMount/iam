package rest

import (
	"time"

	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
	authhandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/handler"
	authzhandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/handler"
	uchandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/handler"
	idphandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/idp/handler"
	authnMiddleware "github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

// RouterOptions carries transport bootstrap decisions into the REST router.
type RouterOptions struct {
	DebugCacheGovernance DebugCacheGovernanceOptions
	SeedMockAuth         SeedMockAuthOptions
}

// DebugCacheGovernanceOptions keeps nil as "unset" so router defaults remain
// distinguishable from explicit false.
type DebugCacheGovernanceOptions struct {
	AppMode      string
	Enabled      *bool
	RequireAdmin *bool
}

// SeedMockAuthOptions controls the internal seed mock route.
type SeedMockAuthOptions struct {
	Enabled      bool
	SharedSecret string
}

// Deps is the concrete dependency surface consumed by the REST router.
type Deps struct {
	Authn           AuthnDeps
	Authz           AuthzDeps
	IDP             IDPDeps
	User            UserDeps
	Suggest         SuggestDeps
	CacheGovernance *cachegovernance.ReadService
	ModuleStatus    ModuleStatus
	RouterOptions
}

type AuthnDeps struct {
	AuthHandler         *authhandler.AuthHandler
	AccountHandler      *authhandler.AccountHandler
	JWKSHandler         *authhandler.JWKSHandler
	SessionAdminHandler *authhandler.SessionAdminHandler
	TokenService        tokenapp.TokenApplicationService
}

type AuthzDeps struct {
	RoleHandler        *authzhandler.RoleHandler
	RoleBindingHandler *authzhandler.RoleBindingHandler
	PolicyHandler      *authzhandler.PolicyHandler
	ResourceHandler    *authzhandler.ResourceHandler
	CheckHandler       *authzhandler.CheckHandler
	RouteAuthorization authnMiddleware.RouteAuthorizationRuntime
	HealthReporter     AuthzHealthReporter
}

type IDPDeps struct {
	WechatAppHandler *idphandler.WechatAppHandler
}

type UserDeps struct {
	UserHandler        *uchandler.UserHandler
	ProfileHandler     *uchandler.ProfileHandler
	ProfileLinkHandler *uchandler.ProfileLinkHandler
}

type SuggestDeps struct {
	Service appsuggest.ProfileSuggestor
}

// ModuleStatus is the route-visible module state used by /debug/modules and /health.
type ModuleStatus struct {
	ContainerInitialized bool
	Authn                bool
	Authz                bool
	User                 bool
	IDP                  bool
	Suggest              bool
	AuthEnabled          bool
}

// AuthzHealthReporter exposes authz runtime reload health without leaking the
// concrete infra adapter into the router.
type AuthzHealthReporter interface {
	ReloadHealth() (bool, error, time.Time)
}
