// Package restful IDP 模块 REST API 路由注册
package idp

import (
	"net/http"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/gin-gonic/gin"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/idp/handler"
)

// Dependencies IDP 模块的依赖
type Dependencies struct {
	WechatAppHandler   *handler.WechatAppHandler
	AuthMiddleware     gin.HandlerFunc
	PermissionOrGlobal func(resource, action string) gin.HandlerFunc
	// WechatAuthHandler 已移除 - 认证功能由 authn 模块统一提供
}

// Register 注册 IDP 模块的所有路由
//
// IDP 模块职责：
// - 微信应用管理（创建、查询、凭据轮换、令牌管理）
// - 提供基础设施服务供其他模块使用（通过容器依赖注入）
//
// 认证功能由 AuthN 的 POST /api/v2/authn/login 统一提供。
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil {
		return
	}

	idpGroup := engine.Group("/api/v2/idp")
	{
		// 健康检查
		idpGroup.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
				"module": "idp",
			})
		})

		// 如果依赖未初始化，只注册健康检查
		if deps.WechatAppHandler == nil {
			return
		}

		// ============ 微信应用管理 ============
		wechatApps := idpGroup.Group("/wechat-apps")
		if deps.AuthMiddleware == nil || deps.PermissionOrGlobal == nil {
			log.Warn("IDP wechat-app management routes are not registered because admin middlewares are unavailable")
			return
		}
		wechatApps.Use(deps.AuthMiddleware)
		{
			// 列表查询
			wechatApps.GET("", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, authzapp.ActionList), deps.WechatAppHandler.ListWechatApps)

			// 创建微信应用
			wechatApps.POST("", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, authzapp.ActionCreate), deps.WechatAppHandler.CreateWechatApp)

			// 查询微信应用
			wechatApps.GET("/:app_id", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, authzapp.ActionRead), deps.WechatAppHandler.GetWechatApp)

			// 更新微信应用基础信息
			wechatApps.PATCH("/:app_id", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, authzapp.ActionUpdate), deps.WechatAppHandler.UpdateWechatApp)

			// 启用/禁用微信应用
			wechatApps.POST("/:app_id/enable", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, "enable"), deps.WechatAppHandler.EnableWechatApp)
			wechatApps.POST("/:app_id/disable", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, "disable"), deps.WechatAppHandler.DisableWechatApp)

			// 获取访问令牌
			wechatApps.GET("/:app_id/access-token", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, "get_access_token"), deps.WechatAppHandler.GetAccessToken)

			// 轮换认证密钥
			wechatApps.POST("/rotate-auth-secret", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, "rotate_auth_secret"), deps.WechatAppHandler.RotateAuthSecret)

			// 轮换消息密钥
			wechatApps.POST("/rotate-msg-secret", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, "rotate_msg_secret"), deps.WechatAppHandler.RotateMsgSecret)

			// 刷新访问令牌
			wechatApps.POST("/refresh-access-token", deps.PermissionOrGlobal(authzapp.ResourceWechatApps, "refresh_access_token"), deps.WechatAppHandler.RefreshAccessToken)
		}
	}
}
