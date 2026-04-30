package authz

import (
	"net/http"

	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/handler"
	"github.com/gin-gonic/gin"
)

// Dependencies 授权模块的依赖
type Dependencies struct {
	RoleHandler        *handler.RoleHandler
	RoleBindingHandler *handler.RoleBindingHandler
	PolicyHandler      *handler.PolicyHandler
	ResourceHandler    *handler.ResourceHandler
	CheckHandler       *handler.CheckHandler
	// AuthMiddleware 保护除 /health 外的管理面与 PDP；若为空则不注册受保护路由。
	AuthMiddleware gin.HandlerFunc
}

// Register 注册授权模块的所有路由
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil {
		return
	}

	authzGroup := engine.Group("/api/v1/authz")
	{
		authzGroup.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
				"module": "authz",
			})
		})

		if deps.RoleHandler == nil {
			return
		}

		if deps.AuthMiddleware == nil {
			return
		}
		g := authzGroup.Group("")
		g.Use(deps.AuthMiddleware)

		// PDP：策略判定
		if deps.CheckHandler != nil {
			g.POST("/check", deps.CheckHandler.Check)
		}

		// ============ 角色管理 ============
		roles := g.Group("/roles")
		{
			roles.POST("", deps.RoleHandler.CreateRole)
			roles.PUT("/:id", deps.RoleHandler.UpdateRole)
			roles.DELETE("/:id", deps.RoleHandler.DeleteRole)
			roles.GET("/:id", deps.RoleHandler.GetRole)
			roles.GET("", deps.RoleHandler.ListRoles)
			roles.GET("/:id/assignments", deps.RoleBindingHandler.ListRoleBindingsByRole)
			roles.GET("/:id/policies", deps.PolicyHandler.GetPoliciesByRole)
		}

		assignments := g.Group("/assignments")
		{
			assignments.POST("/grant", deps.RoleBindingHandler.GrantRoleBinding)
			assignments.POST("/revoke", deps.RoleBindingHandler.RevokeRoleBinding)
			assignments.DELETE("/:id", deps.RoleBindingHandler.RevokeRoleBindingByID)
			assignments.GET("/subject", deps.RoleBindingHandler.ListRoleBindingsBySubject)
		}

		policies := g.Group("/policies")
		{
			policies.POST("", deps.PolicyHandler.AddPermission)
			policies.DELETE("", deps.PolicyHandler.RemovePermission)
			policies.GET("/version", deps.PolicyHandler.GetCurrentVersion)
		}

		resources := g.Group("/resources")
		{
			resources.POST("", deps.ResourceHandler.CreateResource)
			resources.PUT("/:id", deps.ResourceHandler.UpdateResource)
			resources.DELETE("/:id", deps.ResourceHandler.DeleteResource)
			resources.GET("/:id", deps.ResourceHandler.GetResource)
			resources.GET("/key/:key", deps.ResourceHandler.GetResourceByKey)
			resources.GET("", deps.ResourceHandler.ListResources)
			resources.POST("/validate-action", deps.ResourceHandler.ValidateAction)
		}
	}
}
