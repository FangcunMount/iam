package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	authnMiddleware "github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

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

func (r *Router) debugRoutes(c *gin.Context) {
	if r.engine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine not initialized"})
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
	c.JSON(http.StatusOK, gin.H{
		"total":  len(routes),
		"routes": routeList,
	})
}

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

	c.JSON(http.StatusOK, response)
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
