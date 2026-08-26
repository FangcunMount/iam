package authz

import (
	"testing"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterBindsAuthzRoutesToExplicitPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	captured := make(map[string]int)
	permission := func(resource, action string) gin.HandlerFunc {
		captured[resource+"/"+action]++
		return func(c *gin.Context) { c.Next() }
	}
	Register(gin.New(), Dependencies{
		RoleHandler:            handler.NewRoleHandler(nil, nil),
		RoleBindingHandler:     handler.NewRoleBindingHandler(nil, nil),
		PermissionGrantHandler: handler.NewPermissionGrantHandler(nil),
		RoleInheritanceHandler: handler.NewRoleInheritanceHandler(nil),
		ResourceHandler:        handler.NewResourceHandler(nil, nil),
		AuthMiddleware:         func(c *gin.Context) { c.Next() },
		PermissionOrGlobal:     permission,
	})

	for _, key := range []string{
		authzapp.ResourcePermissionGrants + "/" + authzapp.ActionList,
		authzapp.ResourcePermissionGrants + "/" + authzapp.ActionCreate,
		authzapp.ResourcePermissionGrants + "/" + authzapp.ActionRevoke,
		authzapp.ResourceRoleInheritances + "/" + authzapp.ActionList,
		authzapp.ResourceRoleInheritances + "/" + authzapp.ActionGrant,
		authzapp.ResourceRoleInheritances + "/" + authzapp.ActionRevoke,
		authzapp.ResourceAssignments + "/" + authzapp.ActionList,
		authzapp.ResourceAssignments + "/" + authzapp.ActionGrant,
		authzapp.ResourceAssignments + "/" + authzapp.ActionRevoke,
	} {
		require.Positive(t, captured[key], "missing route permission %s", key)
	}
	for key := range captured {
		require.NotContains(t, key, "collection:policies")
		require.NotContains(t, key, "action:check")
	}
}
