package apiserver

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/FangcunMount/component-base/pkg/log"
	openapiFS "github.com/FangcunMount/iam/api"
	"github.com/FangcunMount/iam/internal/apiserver/container"
	authnhttp "github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful"
	authzhttp "github.com/FangcunMount/iam/internal/apiserver/interface/authz/restful"
	cachegovernancehandler "github.com/FangcunMount/iam/internal/apiserver/interface/cachegovernance/restful/handler"
	idphttp "github.com/FangcunMount/iam/internal/apiserver/interface/idp/restful"
	suggesthttp "github.com/FangcunMount/iam/internal/apiserver/interface/suggest/restful"
	userhttp "github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful"
	authnMiddleware "github.com/FangcunMount/iam/internal/pkg/middleware/authn"
	swaggerui "github.com/FangcunMount/iam/web/swagger-ui"
)

// Router 集中的路由管理器
type Router struct {
	container              *container.Container
	engine                 *gin.Engine // 保存 engine 引用用于调试
	cacheGovernanceHandler *cachegovernancehandler.GovernanceHandler
}

// NewRouter 创建路由管理器
func NewRouter(c *container.Container) *Router {
	var governanceHandler = cachegovernancehandler.NewGovernanceHandler(nil)
	if c != nil {
		governanceHandler = cachegovernancehandler.NewGovernanceHandler(c.CacheGovernanceService)
	}
	return &Router{
		container:              c,
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

	if r.container == nil {
		r.registerCacheGovernanceDebugRoutes(engine, nil)
		fmt.Printf("⚠️  container not initialized, skipped module route registration\n")
		return
	}

	// 创建新的认证中间件（可选注入 authz Casbin 以启用 RequireRole / RequirePermission）
	var authMiddleware *authnMiddleware.JWTAuthMiddleware
	if r.container.AuthnModule != nil && r.container.AuthnModule.TokenService != nil {
		var casbin authnMiddleware.CasbinEnforcer
		if r.container.AuthzModule != nil && r.container.AuthzModule.CasbinAdapter != nil {
			casbin = r.container.AuthzModule.CasbinAdapter
		}
		authMiddleware = authnMiddleware.NewJWTAuthMiddleware(
			r.container.AuthnModule.TokenService,
			casbin,
		)
	} else {
		log.Warn("Authn module unavailable; protected routes will not be registered")
	}

	r.registerCacheGovernanceDebugRoutes(engine, authMiddleware)

	adminMiddlewares := make([]gin.HandlerFunc, 0, 2)
	if authMiddleware != nil && authMiddleware.SupportsRoleCheck() {
		adminMiddlewares = append(adminMiddlewares, authMiddleware.AuthRequired(), authMiddleware.RequirePlatformAdmin())
	}

	// Authn 模块（公开端点）
	if r.container.AuthnModule != nil {
		authnhttp.Provide(authnhttp.Dependencies{
			AuthHandler:      r.container.AuthnModule.AuthHandler,
			AccountHandler:   r.container.AuthnModule.AccountHandler,
			JWKSHandler:      r.container.AuthnModule.JWKSHandler,
			AdminMiddlewares: adminMiddlewares,
		})
		authnhttp.Register(engine)
		if viper.GetBool("seed_mock_auth.enabled") {
			secret := strings.TrimSpace(viper.GetString("seed_mock_auth.shared_secret"))
			if secret == "" {
				log.Warn("⚠️  seed_mock_auth.enabled=true but seed_mock_auth.shared_secret is empty; internal mock-consumer route not registered")
			} else {
				authnhttp.RegisterSeedMock(engine, secret)
				log.Info("✅ Authn seed mock routes registered")
			}
		}
		log.Info("✅ Authn module routes registered")
	} else {
		log.Warn("⚠️  Authn module not initialized, routes not registered")
	}

	// Authz 模块（授权管理 + PDP）
	if r.container.AuthzModule != nil && authMiddleware != nil {
		authzhttp.Provide(authzhttp.Dependencies{
			RoleHandler:       r.container.AuthzModule.RoleHandler,
			AssignmentHandler: r.container.AuthzModule.AssignmentHandler,
			PolicyHandler:     r.container.AuthzModule.PolicyHandler,
			ResourceHandler:   r.container.AuthzModule.ResourceHandler,
			CheckHandler:      r.container.AuthzModule.CheckHandler,
			AuthMiddleware:    authMiddleware.AuthRequired(),
		})
		authzhttp.Register(engine)
		log.Info("✅ Authz module routes registered")
	} else if r.container.AuthzModule != nil {
		log.Warn("⚠️  Authz module initialized but JWT middleware unavailable; protected routes not registered")
	} else {
		log.Warn("⚠️  Authz module not initialized, routes not registered")
	}

	// IDP 模块（身份提供者）
	if r.container.IDPModule != nil {
		idphttp.Provide(idphttp.Dependencies{
			WechatAppHandler: r.container.IDPModule.WechatAppHandler,
			AdminMiddlewares: adminMiddlewares,
			// WechatAuthHandler 已移除 - 认证由 authn 模块统一提供
		})
		idphttp.Register(engine)
		log.Info("✅ IDP module routes registered")
	} else {
		log.Warn("⚠️  IDP module not initialized, routes not registered")
	}

	// User 模块（受 JWT 保护）
	if r.container.UserModule != nil && authMiddleware != nil {
		userhttp.Provide(userhttp.Dependencies{
			Module:         r.container.UserModule,
			AuthMiddleware: authMiddleware.AuthRequired(),
		})
		userhttp.Register(engine)
		log.Info("✅ User module routes registered")
	} else if r.container.UserModule != nil {
		log.Warn("⚠️  User module initialized but JWT middleware unavailable; protected routes not registered")
	} else {
		log.Warn("⚠️  User module not initialized, routes not registered")
	}

	// Suggest 模块（依赖 Service 和可选认证）
	if r.container.SuggestModule != nil && r.container.SuggestModule.Service != nil && authMiddleware != nil {
		suggesthttp.Provide(suggesthttp.Dependencies{
			Service:        r.container.SuggestModule.Service,
			AuthMiddleware: authMiddleware.AuthRequired(),
		})
		suggesthttp.Register(engine)
		log.Info("✅ Suggest module routes registered")
	} else if r.container.SuggestModule != nil && r.container.SuggestModule.Service != nil {
		log.Warn("⚠️  Suggest module initialized but JWT middleware unavailable; protected routes not registered")
	} else {
		log.Warn("⚠️  Suggest module not initialized or disabled, routes not registered")
	}

	r.registerAdminRoutes(engine, authMiddleware)

	log.Info("🔗 All routes registration completed")
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
	if viper.IsSet("debug.cache_governance.enabled") {
		return viper.GetBool("debug.cache_governance.enabled")
	}
	return viper.GetString("app.mode") != "production"
}

func (r *Router) cacheGovernanceDebugRequireAdmin() bool {
	if viper.GetString("app.mode") == "production" {
		return true
	}
	if viper.IsSet("debug.cache_governance.require_admin") {
		return viper.GetBool("debug.cache_governance.require_admin")
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
		if r.container != nil && r.container.AuthnModule != nil && r.container.AuthnModule.SessionAdminHandler != nil {
			admin.POST("/sessions/:sessionId/revoke", r.container.AuthnModule.SessionAdminHandler.RevokeSession)
			admin.POST("/accounts/:accountId/sessions/revoke", r.container.AuthnModule.SessionAdminHandler.RevokeAccountSessions)
			admin.POST("/users/:userId/sessions/revoke", r.container.AuthnModule.SessionAdminHandler.RevokeUserSessions)
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
	authEnabled := r.container != nil && r.container.AuthnModule != nil && r.container.AuthnModule.TokenService != nil
	authzRuntimeHealthy := true
	authzRuntimeError := ""
	var authzReloadedAt time.Time

	if r.container != nil && r.container.AuthzModule != nil && r.container.AuthzModule.CasbinAdapter != nil {
		if reporter, ok := r.container.AuthzModule.CasbinAdapter.(interface {
			ReloadHealth() (bool, error, time.Time)
		}); ok {
			var err error
			authzRuntimeHealthy, err, authzReloadedAt = reporter.ReloadHealth()
			if err != nil {
				authzRuntimeError = err.Error()
			}
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
		"container_initialized": r.container != nil,
	}

	if r.container != nil {
		response["modules"] = gin.H{
			"authn": r.container.AuthnModule != nil,
			"authz": r.container.AuthzModule != nil,
			"user":  r.container.UserModule != nil,
			"idp":   r.container.IDPModule != nil,
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
