package identity

import (
	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/handler"
)

// Dependencies bundles the runtime collaborators required by the identity HTTP adapters.
type Dependencies struct {
	UserHandler        *handler.UserHandler
	ProfileHandler     *handler.ProfileHandler
	ProfileLinkHandler *handler.ProfileLinkHandler
	AuthMiddleware     gin.HandlerFunc
}

// Register exposes the identity module REST endpoints on the supplied engine.
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil || deps.AuthMiddleware == nil {
		return
	}

	api := engine.Group("/api/v2/identity")
	api.Use(deps.AuthMiddleware)

	registerUserRoutes(api, deps.UserHandler)
	registerProfileRoutes(api, deps.ProfileHandler)
	registerProfileLinkRoutes(api, deps.ProfileLinkHandler)
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

func registerProfileRoutes(api *gin.RouterGroup, h *handler.ProfileHandler) {
	if h == nil {
		return
	}

	me := api.Group("/me")
	{
		me.GET("/profiles", h.ListMyProfiles)
	}

	api.GET("/profiles/search", h.SearchProfiles)

	profiles := api.Group("/profiles")
	{
		profiles.GET("/:id", h.GetProfile)
		profiles.PATCH("/:id", h.PatchProfile)
	}
}

func registerProfileLinkRoutes(api *gin.RouterGroup, h *handler.ProfileLinkHandler) {
	if h == nil {
		return
	}

	api.GET("/profile-links", h.List)
}
