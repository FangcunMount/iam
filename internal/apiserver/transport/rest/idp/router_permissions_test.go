package idp

import (
	"testing"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/idp/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterBindsWechatAppRoutesToExplicitActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	captured := make(map[string]int)
	Register(gin.New(), Dependencies{
		WechatAppHandler: handler.NewWechatAppHandler(nil, nil, nil),
		AuthMiddleware:   func(c *gin.Context) { c.Next() },
		PermissionOrGlobal: func(resource, action string) gin.HandlerFunc {
			captured[resource+"/"+action]++
			return func(c *gin.Context) { c.Next() }
		},
	})
	for _, action := range []string{
		authzapp.ActionList, authzapp.ActionCreate, authzapp.ActionRead, authzapp.ActionUpdate,
		"enable", "disable", "get_access_token", "rotate_auth_secret", "rotate_msg_secret", "refresh_access_token",
	} {
		require.Positive(t, captured[authzapp.ResourceWechatApps+"/"+action], "missing wechat app action %s", action)
	}
}
