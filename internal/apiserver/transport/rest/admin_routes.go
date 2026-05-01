package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	authnMiddleware "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
)

func (r *Router) registerAdminRoutes(engine *gin.Engine, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if engine == nil {
		return
	}
	if authMiddleware == nil || !authMiddleware.SupportsRoleCheck() {
		log.Warn("Skip admin routes: admin protection middleware is unavailable")
		return
	}

	apiV2 := engine.Group("/api/v2")
	apiV2.Use(authMiddleware.AuthRequired(), authMiddleware.RequirePlatformAdmin())

	admin := apiV2.Group("/admin")
	{
		admin.GET("/users", r.placeholder)
		admin.GET("/statistics", r.placeholder)
		admin.GET("/logs", r.placeholder)
		if r.deps.Authn.SessionAdminHandler != nil {
			admin.POST("/sessions/:sessionId/revoke", r.deps.Authn.SessionAdminHandler.RevokeSession)
			admin.POST("/accounts/:accountId/sessions/revoke", r.deps.Authn.SessionAdminHandler.RevokeAccountSessions)
			admin.POST("/users/:userId/sessions/revoke", r.deps.Authn.SessionAdminHandler.RevokeUserSessions)
		}
	}
}

func (r *Router) placeholder(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"code":    501,
		"message": "功能尚未实现",
		"path":    c.Request.URL.Path,
		"method":  c.Request.Method,
	})
}
