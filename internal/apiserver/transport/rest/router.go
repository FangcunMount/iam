package rest

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	cachegovernancehandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/cachegovernance/handler"
	authnMiddleware "github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

// Router 集中的路由管理器
type Router struct {
	deps                   Deps
	engine                 *gin.Engine // 保存 engine 引用用于调试
	cacheGovernanceHandler *cachegovernancehandler.GovernanceHandler
}

type routeDependencies struct {
	authn            AuthnDeps
	authz            AuthzDeps
	idp              IDPDeps
	user             UserDeps
	suggest          SuggestDeps
	authMiddleware   *authnMiddleware.JWTAuthMiddleware
	adminMiddlewares []gin.HandlerFunc
}

// NewRouter 创建路由管理器
func NewRouter(deps Deps) *Router {
	governanceHandler := cachegovernancehandler.NewGovernanceHandler(deps.CacheGovernance)
	return &Router{
		deps:                   deps,
		cacheGovernanceHandler: governanceHandler,
	}
}

// RegisterRoutes 注册所有路由
func (r *Router) RegisterRoutes(engine *gin.Engine) {
	if engine == nil {
		return
	}

	r.engine = engine // 保存引用用于调试

	r.registerBaseRoutes(engine)

	if !r.deps.ModuleStatus.ContainerInitialized {
		r.registerCacheGovernanceDebugRoutes(engine, nil)
		fmt.Printf("⚠️  container not initialized, skipped module route registration\n")
		return
	}

	deps := r.resolveRouteDependencies()
	authMiddleware := deps.authMiddleware
	if authMiddleware == nil {
		log.Warn("Authn module unavailable; protected routes will not be registered")
	}

	r.registerCacheGovernanceDebugRoutes(engine, authMiddleware)

	r.registerModuleRoutes(engine, deps, authMiddleware)
	r.registerAdminRoutes(engine, authMiddleware)

	log.Info("🔗 All routes registration completed")
}

func (r *Router) resolveRouteDependencies() routeDependencies {
	if !r.deps.ModuleStatus.ContainerInitialized {
		return routeDependencies{}
	}

	deps := routeDependencies{
		authn:   r.deps.Authn,
		authz:   r.deps.Authz,
		idp:     r.deps.IDP,
		user:    r.deps.User,
		suggest: r.deps.Suggest,
	}

	if deps.authn.TokenService != nil {
		deps.authMiddleware = authnMiddleware.NewJWTAuthMiddleware(deps.authn.TokenService, deps.authz.Casbin)
	}

	if deps.authMiddleware != nil && deps.authMiddleware.SupportsRoleCheck() {
		deps.adminMiddlewares = append(deps.adminMiddlewares, deps.authMiddleware.AuthRequired(), deps.authMiddleware.RequirePlatformAdmin())
	}

	return deps
}
