package authn

import (
	"strings"

	"github.com/gin-gonic/gin"

	authhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/handler"
	authnMiddleware "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
)

// Dependencies describes the external collaborators needed to expose authn endpoints.
type Dependencies struct {
	AuthHandler      *authhandler.AuthHandler    // 新的认证处理器
	AccountHandler   *authhandler.AccountHandler // 账户管理处理器
	JWKSHandler      *authhandler.JWKSHandler    // JWKS 处理器
	AdminMiddlewares []gin.HandlerFunc           // 管理接口中间件
}

// Register exposes the authentication endpoints that issue and refresh tokens.
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil {
		return
	}

	api := engine.Group("/api/v2/authn")

	// 注册符合 v2 API 文档的认证端点
	registerAuthEndpoints(api.Group(""), deps.AuthHandler)

	// 注册账户管理端点
	registerAccountEndpoints(api.Group(""), deps.AccountHandler)

	// 注册 JWKS 端点（公开端点）
	registerJWKSPublicEndpoints(engine, deps.JWKSHandler)

	// 注册 JWKS 管理端点（管理员接口）
	registerJWKSAdminEndpoints(api.Group("/admin"), deps.JWKSHandler, deps.AdminMiddlewares...)
}

// RegisterSeedMock exposes the internal mock-consumer ensure endpoint when explicitly enabled.
func RegisterSeedMock(engine *gin.Engine, accountHandler *authhandler.AccountHandler, sharedSecret string) {
	if engine == nil || accountHandler == nil {
		return
	}
	sharedSecret = strings.TrimSpace(sharedSecret)
	if sharedSecret == "" {
		return
	}

	internal := engine.Group("/api/v2/internal/authn")
	internal.Use(authnMiddleware.RequireSeedMockSecret(sharedSecret))
	registerInternalMockConsumerEndpoints(internal, accountHandler)
}

// registerAuthEndpoints 注册 v2 显式认证端点。
func registerAuthEndpoints(group *gin.RouterGroup, handler *authhandler.AuthHandler) {
	if group == nil || handler == nil {
		return
	}

	// 认证端点(符合 API 文档)
	group.POST("/login", handler.LoginV2)
	// 登录预准备（发码、未来扫码会话等）
	group.POST("/login/prep/phone-otp", handler.PreparePhoneOTPLogin)
	group.POST("/refresh_token", handler.RefreshToken)
	group.POST("/logout", handler.Logout)
	group.POST("/verify", handler.VerifyToken)
}

// registerJWKSPublicEndpoints 注册 JWKS 公开端点
func registerJWKSPublicEndpoints(engine *gin.Engine, handler *authhandler.JWKSHandler) {
	if engine == nil || handler == nil {
		return
	}

	// JWKS 公开端点（无需认证）
	engine.GET("/.well-known/jwks.json", handler.GetJWKS)
	engine.GET("/api/v2/.well-known/jwks.json", handler.GetJWKS)
}

// registerJWKSAdminEndpoints 注册 JWKS 管理端点
func registerJWKSAdminEndpoints(admin *gin.RouterGroup, handler *authhandler.JWKSHandler, middlewares ...gin.HandlerFunc) {
	if admin == nil || handler == nil || len(middlewares) == 0 {
		return
	}
	admin.Use(middlewares...)

	// JWKS 管理端点（需要管理员权限）
	jwks := admin.Group("/jwks")
	{
		// 密钥管理
		jwks.POST("/keys", handler.CreateKey)                        // 创建密钥
		jwks.GET("/keys", handler.ListKeys)                          // 列出密钥
		jwks.GET("/keys/:kid", handler.GetKey)                       // 获取密钥详情
		jwks.POST("/keys/:kid/retire", handler.RetireKey)            // 退役密钥
		jwks.POST("/keys/:kid/force-retire", handler.ForceRetireKey) // 强制退役密钥
		jwks.POST("/keys/:kid/grace", handler.EnterGracePeriod)      // 进入宽限期
		jwks.POST("/keys/cleanup", handler.CleanupExpiredKeys)       // 清理过期密钥
		jwks.GET("/keys/publishable", handler.GetPublishableKeys)    // 获取可发布的密钥
	}
}

func registerAccountEndpoints(api *gin.RouterGroup, h *authhandler.AccountHandler) {
	if api == nil || h == nil {
		return
	}

	signups := api.Group("/signups")
	signups.POST("/wechat-miniprogram", h.SignUpWithWeChatMiniProgram)

	// 账户查询和管理（需要认证）
	accounts := api.Group("/accounts")
	accounts.GET("/:accountId", h.GetAccountByID)
	accounts.PUT("/:accountId/profile", h.UpdateProfile)
	accounts.PUT("/:accountId/unionid", h.SetUnionID)
	accounts.POST("/:accountId/enable", h.EnableAccount)
	accounts.POST("/:accountId/disable", h.DisableAccount)
}

func registerInternalMockConsumerEndpoints(api *gin.RouterGroup, h *authhandler.AccountHandler) {
	if api == nil || h == nil {
		return
	}

	mockConsumers := api.Group("/mock-consumers")
	mockConsumers.POST("/ensure", h.EnsureMockConsumer)
}
