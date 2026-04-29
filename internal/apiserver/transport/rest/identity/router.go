package identity

import (
	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/handler"
)

// Dependencies bundles the runtime collaborators required by the UC HTTP adapters.
type Dependencies struct {
	UserHandler         *handler.UserHandler
	ChildHandler        *handler.ChildHandler
	GuardianshipHandler *handler.GuardianshipHandler
	AuthMiddleware      gin.HandlerFunc
}

// Register exposes the UC module REST endpoints on the supplied engine.
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil || deps.AuthMiddleware == nil {
		return
	}

	api := engine.Group("/api/v1/identity")
	api.Use(deps.AuthMiddleware)

	registerUserRoutes(api, deps.UserHandler)
	registerChildRoutes(api, deps.ChildHandler)
	registerGuardianshipRoutes(api, deps.GuardianshipHandler)
}

// registerUserRoutes 注册用户相关路由
func registerUserRoutes(api *gin.RouterGroup, h *handler.UserHandler) {
	if h == nil {
		return
	}

	me := api.Group("/me")
	{
		me.GET("", h.GetUserProfile)
		me.PATCH("", h.PatchUser)
	}
}

func registerChildRoutes(api *gin.RouterGroup, h *handler.ChildHandler) {
	if h == nil {
		return
	}

	me := api.Group("/me")
	{
		me.GET("/children", h.ListMyChildren)
	}

	api.POST("/children/register", h.RegisterChild)
	api.GET("/children/search", h.SearchChildren)

	children := api.Group("/children")
	{
		children.GET("/:id", h.GetChild)
		children.PATCH("/:id", h.PatchChild)
	}
}

func registerGuardianshipRoutes(api *gin.RouterGroup, h *handler.GuardianshipHandler) {
	if h == nil {
		return
	}

	api.GET("/guardians", h.List)
	api.POST("/guardians/grant", h.Grant)
}
