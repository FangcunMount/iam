package apiserver

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	openapiFS "github.com/FangcunMount/iam/api"
	authnhttp "github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful"
	authzhttp "github.com/FangcunMount/iam/internal/apiserver/interface/authz/restful"
	cachegovernancehandler "github.com/FangcunMount/iam/internal/apiserver/interface/cachegovernance/restful/handler"
	idphttp "github.com/FangcunMount/iam/internal/apiserver/interface/idp/restful"
	suggesthttp "github.com/FangcunMount/iam/internal/apiserver/interface/suggest/restful"
	userhttp "github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful"
	resttransport "github.com/FangcunMount/iam/internal/apiserver/transport/rest"
	authnMiddleware "github.com/FangcunMount/iam/internal/pkg/middleware/authn"
	swaggerui "github.com/FangcunMount/iam/web/swagger-ui"
)

// Router 集中的路由管理器
type Router struct {
	deps                   resttransport.Deps
	engine                 *gin.Engine // 保存 engine 引用用于调试
	cacheGovernanceHandler *cachegovernancehandler.GovernanceHandler
}

type routeDependencies struct {
	authn            resttransport.AuthnDeps
	authz            resttransport.AuthzDeps
	idp              resttransport.IDPDeps
	user             resttransport.UserDeps
	suggest          resttransport.SuggestDeps
	authMiddleware   *authnMiddleware.JWTAuthMiddleware
	adminMiddlewares []gin.HandlerFunc
}

// NewRouter 创建路由管理器
func NewRouter(deps resttransport.Deps) *Router {
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

	// Authn 模块（公开端点）
	if authnRoutesAvailable(deps.authn) {
		authnDeps := authnhttp.Dependencies{
			AuthHandler:      deps.authn.AuthHandler,
			AccountHandler:   deps.authn.AccountHandler,
			JWKSHandler:      deps.authn.JWKSHandler,
			AdminMiddlewares: deps.adminMiddlewares,
		}
		authnhttp.Register(engine, authnDeps)
		if r.deps.SeedMockAuth.Enabled {
			secret := strings.TrimSpace(r.deps.SeedMockAuth.SharedSecret)
			if secret == "" {
				log.Warn("⚠️  seed_mock_auth.enabled=true but seed_mock_auth.shared_secret is empty; internal mock-consumer route not registered")
			} else {
				authnhttp.RegisterSeedMock(engine, deps.authn.AccountHandler, secret)
				log.Info("✅ Authn seed mock routes registered")
			}
		}
		log.Info("✅ Authn module routes registered")
	} else {
		log.Warn("⚠️  Authn module not initialized, routes not registered")
	}

	// Authz 模块（授权管理 + PDP）
	if authzRoutesAvailable(deps.authz) && authMiddleware != nil {
		authzhttp.Register(engine, authzhttp.Dependencies{
			RoleHandler:       deps.authz.RoleHandler,
			AssignmentHandler: deps.authz.AssignmentHandler,
			PolicyHandler:     deps.authz.PolicyHandler,
			ResourceHandler:   deps.authz.ResourceHandler,
			CheckHandler:      deps.authz.CheckHandler,
			AuthMiddleware:    authMiddleware.AuthRequired(),
		})
		log.Info("✅ Authz module routes registered")
	} else if authzRoutesAvailable(deps.authz) {
		log.Warn("⚠️  Authz module initialized but JWT middleware unavailable; protected routes not registered")
	} else {
		log.Warn("⚠️  Authz module not initialized, routes not registered")
	}

	// IDP 模块（身份提供者）
	if r.deps.ModuleStatus.IDP {
		idphttp.Register(engine, idphttp.Dependencies{
			WechatAppHandler: deps.idp.WechatAppHandler,
			AdminMiddlewares: deps.adminMiddlewares,
			// WechatAuthHandler 已移除 - 认证由 authn 模块统一提供
		})
		log.Info("✅ IDP module routes registered")
	} else {
		log.Warn("⚠️  IDP module not initialized, routes not registered")
	}

	// User 模块（受 JWT 保护）
	if userRoutesAvailable(deps.user) && authMiddleware != nil {
		userhttp.Register(engine, userhttp.Dependencies{
			UserHandler:         deps.user.UserHandler,
			ChildHandler:        deps.user.ChildHandler,
			GuardianshipHandler: deps.user.GuardianshipHandler,
			AuthMiddleware:      authMiddleware.AuthRequired(),
		})
		log.Info("✅ User module routes registered")
	} else if userRoutesAvailable(deps.user) {
		log.Warn("⚠️  User module initialized but JWT middleware unavailable; protected routes not registered")
	} else {
		log.Warn("⚠️  User module not initialized, routes not registered")
	}

	// Suggest 模块（依赖 Service 和可选认证）
	if deps.suggest.Service != nil && authMiddleware != nil {
		suggesthttp.Register(engine, suggesthttp.Dependencies{
			Service:        deps.suggest.Service,
			AuthMiddleware: authMiddleware.AuthRequired(),
		})
		log.Info("✅ Suggest module routes registered")
	} else if deps.suggest.Service != nil {
		log.Warn("⚠️  Suggest module initialized but JWT middleware unavailable; protected routes not registered")
	} else {
		log.Warn("⚠️  Suggest module not initialized or disabled, routes not registered")
	}

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

func authnRoutesAvailable(deps resttransport.AuthnDeps) bool {
	return deps.AuthHandler != nil || deps.AccountHandler != nil || deps.JWKSHandler != nil
}

func authzRoutesAvailable(deps resttransport.AuthzDeps) bool {
	return deps.RoleHandler != nil || deps.AssignmentHandler != nil || deps.PolicyHandler != nil ||
		deps.ResourceHandler != nil || deps.CheckHandler != nil
}

func userRoutesAvailable(deps resttransport.UserDeps) bool {
	return deps.UserHandler != nil || deps.ChildHandler != nil || deps.GuardianshipHandler != nil
}

func (r *Router) registerBaseRoutes(engine *gin.Engine) {
	engine.GET("/health", r.healthCheck)
	engine.GET("/ping", r.ping)
	engine.GET("/debug/routes", r.debugRoutes)   // 调试端点：列出所有注册的路由
	engine.GET("/debug/modules", r.debugModules) // 调试端点：查看模块状态

	// Swagger UI 路由（默认在开发环境可用）
	// 生产环境建议通过配置控制是否启用
	engine.StaticFS("/openapi", http.FS(openapiFS.RestFS))
	engine.StaticFS("/swagger", http.FS(swaggerui.DistFS)) // 新版 Swagger UI

	publicAPI := engine.Group("/api/v1/public")
	{
		publicAPI.GET("/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service":     "iam-apiserver",
				"version":     "1.0.0",
				"description": "IAM API Server",
				"swagger":     "/swagger/index.html",
			})
		})
	}
	// admin.Use(r.requireAdminRole()) // 需要实现管理员权限检查中间件
}

func (r *Router) registerCacheGovernanceDebugRoutes(engine *gin.Engine, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if engine == nil || !r.cacheGovernanceDebugEnabled() {
		return
	}

	if !r.cacheGovernanceDebugRequireAdmin() {
		engine.GET("/debug/cache-governance/catalog", r.debugCacheCatalog)
		engine.GET("/debug/cache-governance/overview", r.debugCacheOverview)
		engine.GET("/debug/cache-governance/families/:family", r.debugCacheFamily)
		return
	}

	if authMiddleware == nil || !authMiddleware.SupportsRoleCheck() {
		log.Warn("Skip cache governance debug routes: admin protection enabled but authz middleware is unavailable")
		return
	}

	debug := engine.Group("/debug/cache-governance")
	debug.Use(authMiddleware.AuthRequired(), authMiddleware.RequirePlatformAdmin())
	{
		debug.GET("/catalog", r.debugCacheCatalog)
		debug.GET("/overview", r.debugCacheOverview)
		debug.GET("/families/:family", r.debugCacheFamily)
	}
}

func (r *Router) cacheGovernanceDebugEnabled() bool {
	if r.deps.DebugCacheGovernance.Enabled != nil {
		return *r.deps.DebugCacheGovernance.Enabled
	}
	return r.deps.DebugCacheGovernance.AppMode != "production"
}

func (r *Router) cacheGovernanceDebugRequireAdmin() bool {
	if r.deps.DebugCacheGovernance.AppMode == "production" {
		return true
	}
	if r.deps.DebugCacheGovernance.RequireAdmin != nil {
		return *r.deps.DebugCacheGovernance.RequireAdmin
	}
	return false
}

func (r *Router) registerAdminRoutes(engine *gin.Engine, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if engine == nil {
		return
	}
	if authMiddleware == nil || !authMiddleware.SupportsRoleCheck() {
		log.Warn("Skip admin routes: admin protection middleware is unavailable")
		return
	}

	apiV1 := engine.Group("/api/v1")
	apiV1.Use(authMiddleware.AuthRequired(), authMiddleware.RequirePlatformAdmin())

	admin := apiV1.Group("/admin")
	{
		admin.GET("/users", r.placeholder)      // 管理员获取所有用户
		admin.GET("/statistics", r.placeholder) // 系统统计信息
		admin.GET("/logs", r.placeholder)       // 系统日志
		if r.deps.Authn.SessionAdminHandler != nil {
			admin.POST("/sessions/:sessionId/revoke", r.deps.Authn.SessionAdminHandler.RevokeSession)
			admin.POST("/accounts/:accountId/sessions/revoke", r.deps.Authn.SessionAdminHandler.RevokeAccountSessions)
			admin.POST("/users/:userId/sessions/revoke", r.deps.Authn.SessionAdminHandler.RevokeUserSessions)
		}
	}
}

// placeholder 占位符处理器（用于未实现的功能）
func (r *Router) placeholder(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"code":    501,
		"message": "功能尚未实现",
		"path":    c.Request.URL.Path,
		"method":  c.Request.Method,
	})
}

// healthCheck 健康检查处理函数
func (r *Router) healthCheck(c *gin.Context) {
	authEnabled := r.deps.ModuleStatus.AuthEnabled
	authzRuntimeHealthy := true
	authzRuntimeError := ""
	var authzReloadedAt time.Time

	if r.deps.Authz.HealthReporter != nil {
		var err error
		authzRuntimeHealthy, err, authzReloadedAt = r.deps.Authz.HealthReporter.ReloadHealth()
		if err != nil {
			authzRuntimeError = err.Error()
		}
	}

	status := "healthy"
	statusCode := http.StatusOK
	if !authEnabled || !authzRuntimeHealthy {
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	response := gin.H{
		"status":       status,
		"version":      "1.0.0",
		"discovery":    "auto",
		"architecture": "hexagonal",
		"router":       "centralized",
		"auth":         map[bool]string{true: "enabled", false: "disabled"}[authEnabled],
		"components": gin.H{
			"domain":      "user",
			"ports":       "storage",
			"adapters":    "mysql, http",
			"application": "user_service",
		},
		"auth_system": gin.H{
			"type":    "jwt",
			"enabled": authEnabled,
			"module":  "authn (DDD 4-layer)",
		},
		"authz_runtime": gin.H{
			"healthy":    authzRuntimeHealthy,
			"last_error": authzRuntimeError,
			"reloaded_at": func() string {
				if authzReloadedAt.IsZero() {
					return ""
				}
				return authzReloadedAt.Format(time.RFC3339)
			}(),
		},
	}

	c.JSON(statusCode, response)
}

// ping 简单的连通性测试
func (r *Router) ping(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
		"status":  "ok",
		"router":  "centralized",
		"auth":    "enabled",
	})
}

// debugRoutes 调试端点：列出所有注册的路由
func (r *Router) debugRoutes(c *gin.Context) {
	if r.engine == nil {
		c.JSON(500, gin.H{"error": "engine not initialized"})
		return
	}

	routes := r.engine.Routes()
	routeList := make([]gin.H, 0, len(routes))
	for _, route := range routes {
		routeList = append(routeList, gin.H{
			"method": route.Method,
			"path":   route.Path,
		})
	}
	c.JSON(200, gin.H{
		"total":  len(routes),
		"routes": routeList,
	})
}

// debugModules 调试端点：查看模块初始化状态
func (r *Router) debugModules(c *gin.Context) {
	response := gin.H{
		"container_initialized": r.deps.ModuleStatus.ContainerInitialized,
	}

	if r.deps.ModuleStatus.ContainerInitialized {
		response["modules"] = gin.H{
			"authn":   r.deps.ModuleStatus.Authn,
			"authz":   r.deps.ModuleStatus.Authz,
			"user":    r.deps.ModuleStatus.User,
			"idp":     r.deps.ModuleStatus.IDP,
			"suggest": r.deps.ModuleStatus.Suggest,
		}
		response["container_status"] = "initialized"
	} else {
		response["container_status"] = "not_initialized"
	}

	c.JSON(200, response)
}

func (r *Router) debugCacheCatalog(c *gin.Context) {
	r.cacheGovernanceHandler.GetCatalog(c)
}

func (r *Router) debugCacheOverview(c *gin.Context) {
	r.cacheGovernanceHandler.GetOverview(c)
}

func (r *Router) debugCacheFamily(c *gin.Context) {
	r.cacheGovernanceHandler.GetFamily(c)
}
