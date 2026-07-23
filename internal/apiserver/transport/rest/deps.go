package rest

import (
	"time"

	redis "github.com/redis/go-redis/v9"

	linkingapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	signupapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signup"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	authhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/handler"
	authzhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/handler"
	uchandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/handler"
	idphandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/idp/handler"
	authnMiddleware "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
)

// RouterOptions 路由选项
type RouterOptions struct {
	DebugCacheGovernance DebugCacheGovernanceOptions
	SeedMockAuth         SeedMockAuthOptions
}

// DebugCacheGovernanceOptions 调试缓存治理选项
type DebugCacheGovernanceOptions struct {
	Environment  genericapiserver.Environment
	Enabled      *bool
	RequireAdmin *bool
}

// SeedMockAuthOptions 种子模拟认证选项
type SeedMockAuthOptions struct {
	Enabled      bool
	SharedSecret string
}

// Deps	依赖面
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

// AuthnDeps 认证依赖
type AuthnDeps struct {
	AuthHandler            *authhandler.AuthHandler
	OnboardingHandler      *authhandler.OnboardingHandler
	LoginIdentityHandler   *authhandler.LoginIdentityHandler
	WechatOpenLoginHandler *authhandler.WechatOpenLoginAuthorizeHandler
	JWKSHandler            *authhandler.JWKSHandler
	SessionAdminHandler    *authhandler.SessionAdminHandler
	SignupService          signupapp.SignupService
	LoginIdentityLinking   linkingapp.Linker
	TokenService           tokenapp.TokenApplicationService
}

// AuthzDeps 授权依赖
type AuthzDeps struct {
	RoleHandler        *authzhandler.RoleHandler
	RoleBindingHandler *authzhandler.RoleBindingHandler
	PolicyHandler      *authzhandler.PolicyHandler
	ResourceHandler    *authzhandler.ResourceHandler
	CheckHandler       *authzhandler.CheckHandler
	RouteAuthorization authnMiddleware.RouteAuthorizationRuntime
	HealthReporter     AuthzHealthReporter
}

// IDPDeps IDP依赖
type IDPDeps struct {
	WechatAppHandler *idphandler.WechatAppHandler
}

// UserDeps 用户依赖
type UserDeps struct {
	UserHandler        *uchandler.UserHandler
	ProfileHandler     *uchandler.ProfileHandler
	ProfileLinkHandler *uchandler.ProfileLinkHandler
}

// SuggestDeps 建议依赖
type SuggestDeps struct {
	Service     appsuggest.ProfileSuggestor
	RateLimit   appsuggest.RateLimitConfig
	Metrics     appsuggest.SuggestMetrics
	RateLimiter appsuggest.RateLimiter
	RedisClient *redis.Client
}

// ModuleStatus 模块状态，用于/debug/modules和/health
type ModuleStatus struct {
	ContainerInitialized bool
	Container            ModuleState
	Modules              map[string]ModuleState
	Authn                bool
	Authz                bool
	User                 bool
	IDP                  bool
	Suggest              bool
	AuthEnabled          bool
}

// ModuleState describes a bootstrapped module capability without forcing
// transports to infer availability from legacy booleans.
type ModuleState struct {
	Bootstrapped   bool   `json:"bootstrapped"`
	Available      bool   `json:"available"`
	DegradedReason string `json:"degraded_reason,omitempty"`
}

const (
	moduleStateAuthn    = "authn module"
	moduleStateAuthz    = "authz module"
	moduleStateIDP      = "idp module"
	moduleStateIdentity = "identity module"
	moduleStateSuggest  = "suggest module"
)

func (s ModuleStatus) containerAvailable() bool {
	return s.Container.Available
}

func (s ModuleStatus) moduleAvailable(name string) bool {
	state, ok := s.Modules[name]
	return ok && state.Available
}

func (s ModuleStatus) authnAvailable() bool {
	return s.moduleAvailable(moduleStateAuthn)
}

func (s ModuleStatus) authzAvailable() bool {
	return s.moduleAvailable(moduleStateAuthz)
}

func (s ModuleStatus) idpAvailable() bool {
	return s.moduleAvailable(moduleStateIDP)
}

func (s ModuleStatus) identityAvailable() bool {
	return s.moduleAvailable(moduleStateIdentity)
}

func (s ModuleStatus) suggestAvailable() bool {
	return s.moduleAvailable(moduleStateSuggest)
}

// AuthzHealthReporter 授权运行时重载健康报告，不泄露具体的底层适配器到路由
type AuthzHealthReporter interface {
	ReloadHealth() (bool, error, time.Time)
	RuntimeHealthDetails() map[string]any
}
