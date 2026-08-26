package authz

import (
	"net/http"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/handler"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	RoleHandler            *handler.RoleHandler
	RoleBindingHandler     *handler.RoleBindingHandler
	PermissionGrantHandler *handler.PermissionGrantHandler
	RoleInheritanceHandler *handler.RoleInheritanceHandler
	ResourceHandler        *handler.ResourceHandler
	AuthMiddleware         gin.HandlerFunc
	PermissionOrGlobal     func(resource, action string) gin.HandlerFunc
}

func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil {
		return
	}
	authzGroup := engine.Group("/api/v3/authz")
	authzGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "module": "authz"})
	})
	if deps.RoleHandler == nil || deps.AuthMiddleware == nil || deps.PermissionOrGlobal == nil {
		return
	}
	g := authzGroup.Group("")
	g.Use(deps.AuthMiddleware)

	roles := g.Group("/roles")
	roles.POST("", deps.PermissionOrGlobal(authzapp.ResourceRoles, authzapp.ActionCreate), deps.RoleHandler.CreateRole)
	roles.PUT("/:id", deps.PermissionOrGlobal(authzapp.ResourceRoles, authzapp.ActionUpdate), deps.RoleHandler.UpdateRole)
	roles.DELETE("/:id", deps.PermissionOrGlobal(authzapp.ResourceRoles, authzapp.ActionDelete), deps.RoleHandler.DeleteRole)
	roles.GET("/:id", deps.PermissionOrGlobal(authzapp.ResourceRoles, authzapp.ActionRead), deps.RoleHandler.GetRole)
	roles.GET("", deps.PermissionOrGlobal(authzapp.ResourceRoles, authzapp.ActionList), deps.RoleHandler.ListRoles)
	if deps.RoleBindingHandler != nil {
		roles.GET("/:id/assignments", deps.PermissionOrGlobal(authzapp.ResourceAssignments, authzapp.ActionList), deps.RoleBindingHandler.ListRoleBindingsByRole)
	}
	if deps.PermissionGrantHandler != nil {
		roles.GET("/:id/grants", deps.PermissionOrGlobal(authzapp.ResourcePermissionGrants, authzapp.ActionList), deps.PermissionGrantHandler.ListRoleGrants)
		g.POST("/grants", deps.PermissionOrGlobal(authzapp.ResourcePermissionGrants, authzapp.ActionCreate), deps.PermissionGrantHandler.CreateGrant)
		g.DELETE("/grants/:id", deps.PermissionOrGlobal(authzapp.ResourcePermissionGrants, authzapp.ActionRevoke), deps.PermissionGrantHandler.RevokeGrant)
	}
	if deps.RoleInheritanceHandler != nil {
		inheritances := g.Group("/role-inheritances")
		inheritances.POST("", deps.PermissionOrGlobal(authzapp.ResourceRoleInheritances, authzapp.ActionGrant), deps.RoleInheritanceHandler.Create)
		inheritances.GET("", deps.PermissionOrGlobal(authzapp.ResourceRoleInheritances, authzapp.ActionList), deps.RoleInheritanceHandler.List)
		inheritances.DELETE("/:id", deps.PermissionOrGlobal(authzapp.ResourceRoleInheritances, authzapp.ActionRevoke), deps.RoleInheritanceHandler.Revoke)
	}

	if deps.RoleBindingHandler != nil {
		assignments := g.Group("/assignments")
		assignments.POST("/grant", deps.PermissionOrGlobal(authzapp.ResourceAssignments, authzapp.ActionGrant), deps.RoleBindingHandler.GrantRoleBinding)
		assignments.POST("/revoke", deps.PermissionOrGlobal(authzapp.ResourceAssignments, authzapp.ActionRevoke), deps.RoleBindingHandler.RevokeRoleBinding)
		assignments.DELETE("/:id", deps.PermissionOrGlobal(authzapp.ResourceAssignments, authzapp.ActionRevoke), deps.RoleBindingHandler.RevokeRoleBindingByID)
		assignments.GET("/subject", deps.PermissionOrGlobal(authzapp.ResourceAssignments, authzapp.ActionList), deps.RoleBindingHandler.ListRoleBindingsBySubject)
	}

	if deps.ResourceHandler != nil {
		resources := g.Group("/resources")
		resources.POST("", deps.PermissionOrGlobal(authzapp.ResourceResources, authzapp.ActionCreate), deps.ResourceHandler.CreateResource)
		resources.PUT("/:id", deps.PermissionOrGlobal(authzapp.ResourceResources, authzapp.ActionUpdate), deps.ResourceHandler.UpdateResource)
		resources.DELETE("/:id", deps.PermissionOrGlobal(authzapp.ResourceResources, authzapp.ActionDelete), deps.ResourceHandler.DeleteResource)
		resources.GET("/:id", deps.PermissionOrGlobal(authzapp.ResourceResources, authzapp.ActionRead), deps.ResourceHandler.GetResource)
		resources.GET("/key/:key", deps.PermissionOrGlobal(authzapp.ResourceResources, authzapp.ActionRead), deps.ResourceHandler.GetResourceByKey)
		resources.GET("", deps.PermissionOrGlobal(authzapp.ResourceResources, authzapp.ActionList), deps.ResourceHandler.ListResources)
		resources.POST("/validate-action", deps.PermissionOrGlobal(authzapp.ResourceResources, authzapp.ActionValidateAction), deps.ResourceHandler.ValidateAction)
	}
}
