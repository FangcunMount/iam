package authz

import (
	"net/http"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/handler"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	RoleHandler               *handler.RoleHandler
	RoleBindingHandler        *handler.RoleBindingHandler
	PermissionGrantHandler    *handler.PermissionGrantHandler
	RoleInheritanceHandler    *handler.RoleInheritanceHandler
	ResourceHandler           *handler.ResourceHandler
	AuthMiddleware            gin.HandlerFunc
	PermissionOrPlatformAdmin func(resource, action string) gin.HandlerFunc
}

func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil {
		return
	}
	authzGroup := engine.Group("/api/v3/authz")
	authzGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "module": "authz"})
	})
	if deps.RoleHandler == nil || deps.AuthMiddleware == nil || deps.PermissionOrPlatformAdmin == nil {
		return
	}
	g := authzGroup.Group("")
	g.Use(deps.AuthMiddleware)

	roles := g.Group("/roles")
	roles.POST("", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionCreate), deps.RoleHandler.CreateRole)
	roles.PUT("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionUpdate), deps.RoleHandler.UpdateRole)
	roles.DELETE("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionDelete), deps.RoleHandler.DeleteRole)
	roles.GET("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionRead), deps.RoleHandler.GetRole)
	roles.GET("", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionList), deps.RoleHandler.ListRoles)
	if deps.RoleBindingHandler != nil {
		roles.GET("/:id/assignments", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionRead), deps.RoleBindingHandler.ListRoleBindingsByRole)
	}
	if deps.PermissionGrantHandler != nil {
		roles.GET("/:id/grants", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionRead), deps.PermissionGrantHandler.ListRoleGrants)
		g.POST("/grants", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionWrite), deps.PermissionGrantHandler.CreateGrant)
		g.DELETE("/grants/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourcePolicies, authzapp.ActionDelete), deps.PermissionGrantHandler.RevokeGrant)
	}
	if deps.RoleInheritanceHandler != nil {
		inheritances := g.Group("/role-inheritances")
		inheritances.POST("", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionGrant), deps.RoleInheritanceHandler.Create)
		inheritances.GET("", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionRead), deps.RoleInheritanceHandler.List)
		inheritances.DELETE("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceRoles, authzapp.ActionRevoke), deps.RoleInheritanceHandler.Revoke)
	}

	if deps.RoleBindingHandler != nil {
		assignments := g.Group("/assignments")
		assignments.POST("/grant", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionGrant), deps.RoleBindingHandler.GrantRoleBinding)
		assignments.POST("/revoke", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionRevoke), deps.RoleBindingHandler.RevokeRoleBinding)
		assignments.DELETE("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionDelete), deps.RoleBindingHandler.RevokeRoleBindingByID)
		assignments.GET("/subject", deps.PermissionOrPlatformAdmin(authzapp.ResourceAssignments, authzapp.ActionRead), deps.RoleBindingHandler.ListRoleBindingsBySubject)
	}

	if deps.ResourceHandler != nil {
		resources := g.Group("/resources")
		resources.POST("", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionCreate), deps.ResourceHandler.CreateResource)
		resources.PUT("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionUpdate), deps.ResourceHandler.UpdateResource)
		resources.DELETE("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionDelete), deps.ResourceHandler.DeleteResource)
		resources.GET("/:id", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionRead), deps.ResourceHandler.GetResource)
		resources.GET("/key/:key", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionRead), deps.ResourceHandler.GetResourceByKey)
		resources.GET("", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionList), deps.ResourceHandler.ListResources)
		resources.POST("/validate-action", deps.PermissionOrPlatformAdmin(authzapp.ResourceResources, authzapp.ActionValidateAction), deps.ResourceHandler.ValidateAction)
	}
}
