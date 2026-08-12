package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	authnMiddleware "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
)

// registerCacheGovernanceDebugRoutes 注册缓存治理调试路由
func (r *Router) registerCacheGovernanceDebugRoutes(engine *gin.Engine, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	// 如果引擎为空或缓存治理调试未启用，则返回
	if engine == nil || !r.cacheGovernanceDebugEnabled() {
		return
	}

	// 如果缓存治理调试不需要管理员保护，则注册缓存治理调试路由
	if !r.cacheGovernanceDebugRequireAdmin() {
		engine.GET("/debug/cache-governance/catalog", r.debugCacheCatalog)
		engine.GET("/debug/cache-governance/overview", r.debugCacheOverview)
		engine.GET("/debug/cache-governance/families/:family", r.debugCacheFamily)
		return
	}

	// 如果认证中间件不支持角色检查，则跳过缓存治理调试路由
	if authMiddleware == nil || !authMiddleware.SupportsRoleCheck() {
		log.Warn("Skip cache governance debug routes: admin protection enabled but authz middleware is unavailable")
		return
	}

	// 创建缓存治理调试路由组
	debug := engine.Group("/debug/cache-governance")
	debug.Use(authMiddleware.AuthRequired(), authMiddleware.RequirePlatformAdmin())
	{
		// 注册缓存治理目录路由
		debug.GET("/catalog", r.debugCacheCatalog)
		// 注册缓存治理概览路由
		debug.GET("/overview", r.debugCacheOverview)
		// 注册缓存治理家族路由
		debug.GET("/families/:family", r.debugCacheFamily)
	}
}

// cacheGovernanceDebugEnabled 缓存治理调试是否启用
func (r *Router) cacheGovernanceDebugEnabled() bool {
	// 如果缓存治理调试启用不为空，则返回缓存治理调试启用
	if r.deps.DebugCacheGovernance.Enabled != nil {
		return *r.deps.DebugCacheGovernance.Enabled
	}
	// 非生产环境默认启用缓存治理调试。
	return r.deps.DebugCacheGovernance.Environment != genericapiserver.EnvironmentProduction
}

// cacheGovernanceDebugRequireAdmin 缓存治理调试是否需要管理员保护
func (r *Router) cacheGovernanceDebugRequireAdmin() bool {
	// 生产环境始终要求管理员保护。
	if r.deps.DebugCacheGovernance.Environment == genericapiserver.EnvironmentProduction {
		return true
	}
	// 如果缓存治理调试需要管理员保护不为空，则返回缓存治理调试需要管理员保护
	if r.deps.DebugCacheGovernance.RequireAdmin != nil {
		return *r.deps.DebugCacheGovernance.RequireAdmin
	}
	return false
}

// debugRoutes 调试路由
func (r *Router) debugRoutes(c *gin.Context) {
	if r.engine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine not initialized"})
		return
	}

	// 获取当前注册的路由
	routes := r.engine.Routes()
	// 创建路由列表
	routeList := make([]gin.H, 0, len(routes))
	for _, route := range routes {
		routeList = append(routeList, gin.H{
			"method": route.Method,
			"path":   route.Path,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"total":  len(routes),
		"routes": routeList,
	})
}

// debugModules 模块调试路由
func (r *Router) debugModules(c *gin.Context) {
	response := gin.H{
		"container":     r.deps.ModuleStatus.Container,
		"module_states": r.deps.ModuleStatus.Modules,
	}
	if r.deps.ModuleStatus.Container.Bootstrapped {
		response["container_status"] = "initialized"
	} else {
		response["container_status"] = "not_initialized"
	}

	c.JSON(http.StatusOK, response)
}

// debugCacheCatalog 缓存治理目录调试路由
func (r *Router) debugCacheCatalog(c *gin.Context) {
	r.cacheGovernanceHandler.GetCatalog(c)
}

// debugCacheOverview 缓存治理概览调试路由
func (r *Router) debugCacheOverview(c *gin.Context) {
	r.cacheGovernanceHandler.GetOverview(c)
}

// debugCacheFamily 缓存治理家族调试路由
func (r *Router) debugCacheFamily(c *gin.Context) {
	r.cacheGovernanceHandler.GetFamily(c)
}
