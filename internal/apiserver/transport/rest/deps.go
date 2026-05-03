package rest

import (
	"time"

	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	authhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/handler"
	authzhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/handler"
	uchandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/handler"
	idphandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/idp/handler"
	authnMiddleware "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
)

// RouterOptions 路由选项
type RouterOptions struct {
	DebugCacheGovernance DebugCacheGovernanceOptions
	SeedMockAuth         SeedMockAuthOptions
}

// DebugCacheGovernanceOptions 调试缓存治理选项
type DebugCacheGovernanceOptions struct {
	AppMode      string
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
	AuthHandler         *authhandler.AuthHandler
	AccountHandler      *authhandler.AccountHandler
	JWKSHandler         *authhandler.JWKSHandler
	SessionAdminHandler *authhandler.SessionAdminHandler
	TokenService        tokenapp.TokenApplicationService
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
	Service appsuggest.ProfileSuggestor
}

// ModuleStatus 模块状态，用于/debug/modules和/health
type ModuleStatus struct {
	ContainerInitialized bool
	Authn                bool
	Authz                bool
	User                 bool
	IDP                  bool
	Suggest              bool
	AuthEnabled          bool
}

// AuthzHealthReporter 授权运行时重载健康报告，不泄露具体的底层适配器到路由
type AuthzHealthReporter interface {
	ReloadHealth() (bool, error, time.Time)
}
