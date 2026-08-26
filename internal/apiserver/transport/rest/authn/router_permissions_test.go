package authn

import (
	"testing"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterBindsJWKSAdminRoutesToExplicitActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	captured := make(map[string]int)
	Register(gin.New(), Dependencies{
		JWKSHandler:    handler.NewJWKSHandler(nil, nil, nil),
		AuthMiddleware: func(c *gin.Context) { c.Next() },
		PermissionOrGlobal: func(resource, action string) gin.HandlerFunc {
			captured[resource+"/"+action]++
			return func(c *gin.Context) { c.Next() }
		},
	})
	for _, action := range []string{
		authzapp.ActionCreate, authzapp.ActionList, authzapp.ActionRead,
		"retire", "force_retire", "enter_grace", "cleanup", "list_publishable",
	} {
		require.Positive(t, captured[authzapp.ResourceJWKS+"/"+action], "missing JWKS action %s", action)
	}
}
