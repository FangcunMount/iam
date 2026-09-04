package authn

import (
	"strings"

	"github.com/gin-gonic/gin"

	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	authhandler "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/handler"
	authnMiddleware "github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
)

// Dependencies describes the external collaborators needed to expose authn endpoints.
type Dependencies struct {
	AuthHandler            *authhandler.AuthHandler                     // 认证处理器
	OnboardingHandler      *authhandler.OnboardingHandler               // 登录身份开通处理器
	LoginIdentityHandler   *authhandler.LoginIdentityHandler            // 登录身份绑定处理器
	WechatOpenLoginHandler *authhandler.WechatOpenLoginAuthorizeHandler // 微信扫码登录授权处理器（公开）
	JWKSHandler            *authhandler.JWKSHandler                     // JWKS 处理器
	AuthMiddleware         gin.HandlerFunc                              // 当前用户认证中间件
	PermissionOrGlobal     func(resource, action string) gin.HandlerFunc
}

// Register exposes the authentication endpoints that issue and refresh tokens.
func Register(engine *gin.Engine, deps Dependencies) {
	if engine == nil {
		return
	}

	api := engine.Group("/api/v2/authn")

	// 注册符合 v2 API 文档的认证端点
	registerAuthEndpoints(api.Group(""), deps.AuthHandler)

	// 注册微信扫码登录授权端点（公开，无需鉴权）
	registerWechatOpenLoginEndpoints(api.Group(""), deps.WechatOpenLoginHandler)

	// 注册登录身份开通端点
	registerOnboardingEndpoints(api.Group(""), deps.OnboardingHandler)
	registerLoginIdentityEndpoints(api.Group(""), deps.LoginIdentityHandler, deps.AuthMiddleware)

	// 注册 JWKS 端点（公开端点）
	registerJWKSPublicEndpoints(engine, deps.JWKSHandler)

	// 注册 JWKS 管理端点（管理员接口）
	registerJWKSAdminEndpoints(api.Group("/admin"), deps.JWKSHandler, deps.AuthMiddleware, deps.PermissionOrGlobal)
}

// RegisterSeedMock exposes the internal mock-consumer ensure endpoint when explicitly enabled.
func RegisterSeedMock(engine *gin.Engine, onboardingHandler *authhandler.OnboardingHandler, sharedSecret string) {
	if engine == nil || onboardingHandler == nil {
		return
	}
	sharedSecret = strings.TrimSpace(sharedSecret)
	if sharedSecret == "" {
		return
	}

	internal := engine.Group("/api/v2/internal/authn")
	internal.Use(authnMiddleware.RequireSeedMockSecret(sharedSecret))
	registerInternalMockConsumerEndpoints(internal, onboardingHandler)
}

// registerAuthEndpoints 注册 v2 显式认证端点。
func registerAuthEndpoints(group *gin.RouterGroup, handler *authhandler.AuthHandler) {
	if group == nil || handler == nil {
		return
	}

	// 认证端点(符合 API 文档)
	group.POST("/login", handler.LoginV2)
	group.POST("/challenges/phone-otp", handler.SendLoginPhoneOTP)
	group.POST("/refresh_token", handler.RefreshToken)
	group.POST("/logout", handler.Logout)
	group.POST("/verify", handler.VerifyToken)
}

// registerWechatOpenLoginEndpoints 注册微信扫码登录授权端点（公开）。
func registerWechatOpenLoginEndpoints(group *gin.RouterGroup, handler *authhandler.WechatOpenLoginAuthorizeHandler) {
	if group == nil || handler == nil {
		return
	}
	group.POST("/wechat-open/authorize", handler.StartAuthorize)
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
func registerJWKSAdminEndpoints(
	admin *gin.RouterGroup,
	handler *authhandler.JWKSHandler,
	authMiddleware gin.HandlerFunc,
	permissionOrGlobal func(resource, action string) gin.HandlerFunc,
) {
	if admin == nil || handler == nil || authMiddleware == nil || permissionOrGlobal == nil {
		return
	}
	admin.Use(authMiddleware)

	// JWKS 管理端点（需要管理员权限）
	jwks := admin.Group("/jwks")
	{
		// 密钥管理
		jwks.POST("/keys", permissionOrGlobal(authzapp.ResourceJWKS, authzapp.ActionCreate), handler.CreateKey)
		jwks.GET("/keys", permissionOrGlobal(authzapp.ResourceJWKS, authzapp.ActionList), handler.ListKeys)
		jwks.GET("/keys/:kid", permissionOrGlobal(authzapp.ResourceJWKS, authzapp.ActionRead), handler.GetKey)
		jwks.POST("/keys/:kid/retire", permissionOrGlobal(authzapp.ResourceJWKS, "retire"), handler.RetireKey)
		jwks.POST("/keys/:kid/force-retire", permissionOrGlobal(authzapp.ResourceJWKS, "force_retire"), handler.ForceRetireKey)
		jwks.POST("/keys/cleanup", permissionOrGlobal(authzapp.ResourceJWKS, "cleanup"), handler.CleanupExpiredKeys)
		jwks.GET("/keys/publishable", permissionOrGlobal(authzapp.ResourceJWKS, "list_publishable"), handler.GetPublishableKeys)
	}
}

func registerOnboardingEndpoints(api *gin.RouterGroup, h *authhandler.OnboardingHandler) {
	if api == nil || h == nil {
		return
	}

	signups := api.Group("/signups")
	signups.POST("/wechat-miniprogram", h.SignUpWithWeChatMiniProgram)
}

func registerLoginIdentityEndpoints(api *gin.RouterGroup, h *authhandler.LoginIdentityHandler, authMiddleware gin.HandlerFunc) {
	if api == nil || h == nil || authMiddleware == nil {
		return
	}
	identities := api.Group("/login-identities")
	identities.Use(authMiddleware)
	identities.GET("", h.List)
	identities.POST("/phone/challenge", h.SendPhoneLinkChallenge)
	identities.POST("/phone", h.LinkPhone)
	identities.POST("/wechat-miniprogram", h.LinkWechatMiniProgram)
	identities.POST("/wechat-open/authorize", h.StartWechatOpenLink)
	identities.POST("/wechat-open", h.CompleteWechatOpenLink)
	identities.POST("/wecom", h.LinkWecom)
	identities.DELETE("/:id", h.Unlink)
}

func registerInternalMockConsumerEndpoints(api *gin.RouterGroup, h *authhandler.OnboardingHandler) {
	if api == nil || h == nil {
		return
	}

	mockConsumers := api.Group("/mock-consumers")
	mockConsumers.POST("/ensure", h.EnsureMockConsumer)
}
