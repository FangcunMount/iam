package restful

import (
	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/internal/apiserver/container/assembler"
)

// Dependencies bundles the runtime collaborators required by the UC HTTP adapters.
type Dependencies struct {
	Module         *assembler.UserModule
	AuthMiddleware gin.HandlerFunc
}

var deps Dependencies

// Provide wires the dependencies to be used when Register is invoked.
func Provide(d Dependencies) {
	deps = d
}

// Register exposes the UC module REST endpoints on the supplied engine.
func Register(engine *gin.Engine) {
	if engine == nil || deps.Module == nil || deps.AuthMiddleware == nil {
		return
	}

	api := engine.Group("/api/v1/identity")
	api.Use(deps.AuthMiddleware)

	registerUserRoutes(api, deps.Module)
	registerChildRoutes(api, deps.Module)
	registerGuardianshipRoutes(api, deps.Module)
}

// registerUserRoutes 注册用户相关路由
func registerUserRoutes(api *gin.RouterGroup, module *assembler.UserModule) {
	if module.UserHandler == nil {
		return
	}

	me := api.Group("/me")
	{
		me.GET("", module.UserHandler.GetUserProfile)
		me.PATCH("", module.UserHandler.PatchUser)
	}
}

func registerChildRoutes(api *gin.RouterGroup, module *assembler.UserModule) {
	if module.ChildHandler == nil {
		return
	}

	me := api.Group("/me")
	{
		me.GET("/children", module.ChildHandler.ListMyChildren)
	}

	api.POST("/children/register", module.ChildHandler.RegisterChild)
	api.GET("/children/search", module.ChildHandler.SearchChildren)

	children := api.Group("/children")
	{
		children.GET("/:id", module.ChildHandler.GetChild)
		children.PATCH("/:id", module.ChildHandler.PatchChild)
	}
}

func registerGuardianshipRoutes(api *gin.RouterGroup, module *assembler.UserModule) {
	if module.GuardianshipHandler == nil {
		return
	}

	api.GET("/guardians", module.GuardianshipHandler.List)
	api.POST("/guardians/grant", module.GuardianshipHandler.Grant)
}
