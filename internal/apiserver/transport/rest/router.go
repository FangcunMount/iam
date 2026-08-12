package rest

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	cachegovernancehandler "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/cachegovernance/handler"
	authnMiddleware "github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
)

// Router 集中的路由管理器
type Router struct {
	deps                   Deps
	engine                 *gin.Engine // 保存 engine 引用用于调试
	cacheGovernanceHandler *cachegovernancehandler.GovernanceHandler
}

// routeDependencies 路由依赖
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

	// 注册基础路由
	r.registerBaseRoutes(engine)

	// 如果容器未初始化，则注册缓存治理调试路由
	if !r.deps.ModuleStatus.containerAvailable() {
		r.registerCacheGovernanceDebugRoutes(engine, nil)
		fmt.Printf("⚠️  container not initialized, skipped module route registration\n")
		return
	}

	// 解析路由依赖
	deps := r.resolveRouteDependencies()
	authMiddleware := deps.authMiddleware
	if authMiddleware == nil {
		log.Warn("Authn module unavailable; protected routes will not be registered")
	}

	// 注册缓存治理调试路由
	r.registerCacheGovernanceDebugRoutes(engine, authMiddleware)

	// 注册模块路由
	r.registerModuleRoutes(engine, deps, authMiddleware)

	// 注册管理员路由
	r.registerAdminRoutes(engine, authMiddleware)

	log.Info("🔗 All routes registration completed")
}

// resolveRouteDependencies 解析路由依赖
func (r *Router) resolveRouteDependencies() routeDependencies {
	if !r.deps.ModuleStatus.containerAvailable() {
		return routeDependencies{}
	}

	// 创建路由依赖
	deps := routeDependencies{
		authn:   r.deps.Authn,
		authz:   r.deps.Authz,
		idp:     r.deps.IDP,
		user:    r.deps.User,
		suggest: r.deps.Suggest,
	}

	// 创建认证中间件
	if r.deps.ModuleStatus.authnAvailable() && deps.authn.TokenService != nil {
		deps.authMiddleware = authnMiddleware.NewJWTAuthMiddleware(deps.authn.TokenService, deps.authz.RouteAuthorization)
	}

	// 若认证中间件支持角色检查，则拼出管理员中间件
	if deps.authMiddleware != nil && deps.authMiddleware.SupportsRoleCheck() {
		deps.adminMiddlewares = append(deps.adminMiddlewares, deps.authMiddleware.AuthRequired(), deps.authMiddleware.RequirePlatformAdmin())
	}

	// 返回路由依赖
	return deps
}
