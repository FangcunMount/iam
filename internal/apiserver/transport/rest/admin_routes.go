package rest

import (
	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	authnMiddleware "github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
	authzMiddleware "github.com/FangcunMount/iam/v3/internal/pkg/middleware/authz"
)

// registerAdminRoutes 注册管理员路由
func (r *Router) registerAdminRoutes(engine *gin.Engine, authMiddleware *authnMiddleware.JWTAuthMiddleware, authorizationMiddleware *authzMiddleware.Middleware) {
	if engine == nil {
		return
	}

	if authMiddleware == nil || authorizationMiddleware == nil || !authorizationMiddleware.Available() {
		log.Warn("Skip admin routes: admin protection middleware is unavailable")
		return
	}

	// 创建管理员路由组
	apiV2 := engine.Group("/api/v2")
	apiV2.Use(authMiddleware.AuthRequired())

	// 创建管理员路由组
	admin := apiV2.Group("/admin")
	{
		// 如果会话管理器不为空，则注册会话管理路由
		if r.deps.Authn.SessionAdminHandler != nil {
			admin.POST("/sessions/:sessionId/revoke", authorizationMiddleware.RequirePermissionOrGlobal(authzapp.ResourceSessions, authzapp.ActionRevoke), r.deps.Authn.SessionAdminHandler.RevokeSession)
			admin.POST("/login-identities/:loginIdentityId/sessions/revoke", authorizationMiddleware.RequirePermissionOrGlobal(authzapp.ResourceSessions, authzapp.ActionRevokeByLoginIdentity), r.deps.Authn.SessionAdminHandler.RevokeLoginIdentitySessions)
			admin.POST("/users/:userId/sessions/revoke", authorizationMiddleware.RequirePermissionOrGlobal(authzapp.ResourceSessions, authzapp.ActionRevokeByUser), r.deps.Authn.SessionAdminHandler.RevokeUserSessions)
		}
	}
}
