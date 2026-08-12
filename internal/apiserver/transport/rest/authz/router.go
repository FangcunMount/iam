package authz

import (
	"net/http"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/handler"
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
	AuthMiddleware            gin.HandlerFunc
	PermissionOrPlatformAdmin func(resource, action string) gin.HandlerFunc
}

// Register 注册授权模块的所有路由
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil {
		return
	}

	authzGroup := engine.Group("/api/v2/authz")
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

		if deps.AuthMiddleware == nil || deps.PermissionOrPlatformAdmin == nil {
			return
		}
		g := authzGroup.Group("")
		g.Use(deps.AuthMiddleware)

		// PDP：策略判定
		if deps.CheckHandler != nil {
			g.POST("/check",
				deps.PermissionOrPlatformAdmin(authzapp.ResourceCheck, authzapp.ActionCheck),
				deps.CheckHandler.Check,
			)
		}

		// ============ 角色管理 ============
		roles := g.Group("/roles")
		{
			roles.POST("", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionCreate), deps.RoleHandler.CreateRole)
			roles.PUT("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionUpdate), deps.RoleHandler.UpdateRole)
			roles.DELETE("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionDelete), deps.RoleHandler.DeleteRole)
			roles.GET("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionRead), deps.RoleHandler.GetRole)
			roles.GET("", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionList), deps.RoleHandler.ListRoles)
			roles.GET("/:id/assignments", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionRead), deps.RoleBindingHandler.ListRoleBindingsByRole)
			roles.GET("/:id/policies", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionRead), deps.PolicyHandler.GetPoliciesByRole)
		}

		assignments := g.Group("/assignments")
		{
			assignments.POST("/grant", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionGrant), deps.RoleBindingHandler.GrantRoleBinding)
			assignments.POST("/revoke", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionRevoke), deps.RoleBindingHandler.RevokeRoleBinding)
			assignments.DELETE("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionDelete), deps.RoleBindingHandler.RevokeRoleBindingByID)
			assignments.GET("/subject", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionRead), deps.RoleBindingHandler.ListRoleBindingsBySubject)
		}

		policies := g.Group("/policies")
		{
			policies.POST("", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionWrite), deps.PolicyHandler.AddPermission)
			policies.DELETE("", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionDelete), deps.PolicyHandler.RemovePermission)
			policies.GET("/lint", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionRead), deps.PolicyHandler.LintPolicies)
			policies.GET("/version", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionRead), deps.PolicyHandler.GetCurrentVersion)
		}

		resources := g.Group("/resources")
		{
			resources.POST("", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionCreate), deps.ResourceHandler.CreateResource)
			resources.PUT("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionUpdate), deps.ResourceHandler.UpdateResource)
			resources.DELETE("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionDelete), deps.ResourceHandler.DeleteResource)
			resources.GET("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionRead), deps.ResourceHandler.GetResource)
			resources.GET("/key/:key", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionRead), deps.ResourceHandler.GetResourceByKey)
			resources.GET("", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionList), deps.ResourceHandler.ListResources)
			resources.POST("/validate-action", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionValidateAction), deps.ResourceHandler.ValidateAction)
		}
	}
}
