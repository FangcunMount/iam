package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	authnMiddleware "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
)

// registerAdminRoutes 注册管理员路由
func (r *Router) registerAdminRoutes(engine *gin.Engine, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if engine == nil {
		return
	}

	// 如果认证中间件不支持角色检查，则跳过管理员路由
	if authMiddleware == nil || !authMiddleware.SupportsRoleCheck() {
		log.Warn("Skip admin routes: admin protection middleware is unavailable")
		return
	}

	// 创建管理员路由组
	apiV2 := engine.Group("/api/v2")
	apiV2.Use(authMiddleware.AuthRequired(), authMiddleware.RequirePlatformAdmin())

	// 创建管理员路由组
	admin := apiV2.Group("/admin")
	{
		// 注册用户管理路由
		admin.GET("/users", r.placeholder)
		// 注册统计管理路由
		admin.GET("/statistics", r.placeholder)
		// 注册日志管理路由
		admin.GET("/logs", r.placeholder)
		// 如果会话管理器不为空，则注册会话管理路由
		if r.deps.Authn.SessionAdminHandler != nil {
			admin.POST("/sessions/:sessionId/revoke", r.deps.Authn.SessionAdminHandler.RevokeSession)
			admin.POST("/login-identities/:loginIdentityId/sessions/revoke", r.deps.Authn.SessionAdminHandler.RevokeLoginIdentitySessions)
			admin.POST("/users/:userId/sessions/revoke", r.deps.Authn.SessionAdminHandler.RevokeUserSessions)
		}
	}
}

// placeholder 占位路由
func (r *Router) placeholder(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"code":    501,
		"message": "功能尚未实现",
		"path":    c.Request.URL.Path,
		"method":  c.Request.Method,
	})
}
