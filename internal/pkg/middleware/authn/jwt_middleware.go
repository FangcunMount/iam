package authn

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v2/pkg/core"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// RouteAuthorizationRuntime 可选注入路由级授权运行时。
// 为 nil 时 RequireRole / RequirePermission 返回服务不可用。
type RouteAuthorizationRuntime interface {
	AuthorizeRoute(ctx context.Context, sub, tenantID, resourceKey, action string) (bool, error)
	DirectRoleKeys(ctx context.Context, sub, tenantID string) ([]string, error)
}

// JWTAuthMiddleware JWT 认证中间件
// 使用新的认证模块来验证令牌
type JWTAuthMiddleware struct {
	tokenService token.TokenApplicationService
	routeAuth    RouteAuthorizationRuntime
}

// NewJWTAuthMiddleware 创建 JWT 认证中间件。
// routeAuth 可为 nil（仅 JWT 校验，不做角色/权限判定）。
func NewJWTAuthMiddleware(tokenService token.TokenApplicationService, routeAuth RouteAuthorizationRuntime) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		tokenService: tokenService,
		routeAuth:    routeAuth,
	}
}

// SupportsRoleCheck 返回当前中间件是否具备角色判定能力。
func (m *JWTAuthMiddleware) SupportsRoleCheck() bool {
	return m != nil && m.routeAuth != nil
}

// AuthRequired 认证必需中间件
// 验证请求中的 JWT 令牌,如果无效则返回 401
func (m *JWTAuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenValue, source := m.extractToken(c)
		if tokenValue == "" {
			log.Warnw("authorization token missing",
				"path", c.FullPath(),
				"method", c.Request.Method,
				"token_source", source,
				"request_id", requestIDFromContext(c),
			)
			core.WriteResponse(c, errors.WithCode(code.ErrTokenInvalid, "Missing authorization token"), nil)
			c.Abort()
			return
		}

		// 不记录完整 token，仅在中央验证处输出必要信息

		// 验证令牌
		resp, err := m.tokenService.VerifyToken(c.Request.Context(), token.VerifyTokenRequest{
			AccessToken: tokenValue,
		})
		if err != nil {
			log.Errorw("token verification request failed",
				"path", c.FullPath(),
				"method", c.Request.Method,
				"token_source", source,
				"request_id", requestIDFromContext(c),
			)
			core.WriteResponse(c, errors.WithCode(code.ErrTokenInvalid, "Token verification failed"), nil)
			c.Abort()
			return
		}
		if resp == nil || !resp.Valid {
			log.Warnw("token rejected by verification",
				"path", c.FullPath(),
				"method", c.Request.Method,
				"token_source", source,
				"request_id", requestIDFromContext(c),
			)
			core.WriteResponse(c, errors.WithCode(code.ErrTokenInvalid, "Invalid or expired token"), nil)
			c.Abort()
			return
		}

		// 将用户信息存入上下文（从 Claims 中读取）
		if resp.Claims != nil {
			applyVerifiedClaims(c, resp.Claims)
		}

		c.Next()
	}
}

// RequirePlatformAdmin 要求用户在当前 domain 或 platform domain 中具备平台管理员角色。
// 必须在 AuthRequired 之后使用。
func (m *JWTAuthMiddleware) RequirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.routeAuth == nil {
			core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization engine not configured"), nil)
			c.Abort()
			return
		}
		userID, ok := requestctx.UserID(c)
		if !ok {
			core.WriteResponse(c, errors.WithCode(code.ErrUnauthorized, "Not authenticated"), nil)
			c.Abort()
			return
		}

		uid := userID.String()
		sub := "user:" + uid
		domains := []string{requestctx.TenantIDOrDefault(c)}
		if domains[0] != tenant.PlatformID {
			domains = append(domains, tenant.PlatformID)
		}

		for _, dom := range domains {
			roles, err := m.routeAuth.DirectRoleKeys(c.Request.Context(), sub, dom)
			if err != nil {
				log.Errorw("route authorization role lookup failed", "sub", sub, "dom", dom)
				core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization check failed"), nil)
				c.Abort()
				return
			}
			for _, got := range roles {
				if IsPlatformAdminRole(got) {
					c.Next()
					return
				}
			}
		}

		core.WriteResponse(c, errors.WithCode(code.ErrPermissionDenied, "Forbidden"), nil)
		c.Abort()
	}
}

// RequirePermission 对资源键与动作执行路由级授权判定。
// 必须在 AuthRequired 之后使用。
func (m *JWTAuthMiddleware) RequirePermission(resourceObj, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.routeAuth == nil {
			core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization engine not configured"), nil)
			c.Abort()
			return
		}
		userID, ok := requestctx.UserID(c)
		if !ok {
			core.WriteResponse(c, errors.WithCode(code.ErrUnauthorized, "Not authenticated"), nil)
			c.Abort()
			return
		}
		uid := userID.String()
		sub := "user:" + uid
		dom := requestctx.TenantIDOrDefault(c)
		allowed, err := m.routeAuth.AuthorizeRoute(c.Request.Context(), sub, dom, resourceObj, action)
		if err != nil {
			log.Errorw("route authorization failed", "sub", sub, "dom", dom)
			core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization check failed"), nil)
			c.Abort()
			return
		}
		if !allowed {
			core.WriteResponse(c, errors.WithCode(code.ErrPermissionDenied, "Forbidden"), nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePermissionOrPlatformAdmin allows a route when the current tenant
// grants the exact permission or when the subject is a platform administrator.
// Authorization backend failures fail closed unless another branch has already
// positively authorized the request.
func (m *JWTAuthMiddleware) RequirePermissionOrPlatformAdmin(resourceObj, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.routeAuth == nil {
			recordHTTPAuthorization(resourceObj, action, "error")
			core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization engine not configured"), nil)
			c.Abort()
			return
		}
		userID, ok := requestctx.UserID(c)
		if !ok {
			recordHTTPAuthorization(resourceObj, action, "unauthenticated")
			core.WriteResponse(c, errors.WithCode(code.ErrTokenInvalid, "Not authenticated"), nil)
			c.Abort()
			return
		}

		sub := "user:" + userID.String()
		dom := requestctx.TenantIDOrDefault(c)
		allowed, permissionErr := m.routeAuth.AuthorizeRoute(
			c.Request.Context(),
			sub,
			dom,
			resourceObj,
			action,
		)
		if allowed {
			recordHTTPAuthorization(resourceObj, action, "permission")
			c.Next()
			return
		}

		adminAllowed, adminErr := m.isPlatformAdmin(c.Request.Context(), sub, dom)
		if adminAllowed {
			recordHTTPAuthorization(resourceObj, action, "platform_admin")
			c.Next()
			return
		}
		if permissionErr != nil || adminErr != nil {
			recordHTTPAuthorization(resourceObj, action, "error")
			log.Errorw("route authorization check failed",
				"resource", resourceObj,
				"action", action,
			)
			core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization check failed"), nil)
			c.Abort()
			return
		}

		recordHTTPAuthorization(resourceObj, action, "denied")
		core.WriteResponse(c, errors.WithCode(code.ErrPermissionDenied, "Forbidden"), nil)
		c.Abort()
	}
}

func (m *JWTAuthMiddleware) isPlatformAdmin(ctx context.Context, sub, currentDomain string) (bool, error) {
	domains := []string{currentDomain}
	if currentDomain != tenant.PlatformID {
		domains = append(domains, tenant.PlatformID)
	}
	for _, dom := range domains {
		roles, err := m.routeAuth.DirectRoleKeys(ctx, sub, dom)
		if err != nil {
			return false, err
		}
		for _, role := range roles {
			if IsPlatformAdminRole(role) {
				return true, nil
			}
		}
	}
	return false, nil
}

func applyVerifiedClaims(c *gin.Context, claims *token.TokenClaims) {
	if c == nil || claims == nil {
		return
	}

	ctx := context.WithValue(c.Request.Context(), requestctx.KeyUserID, claims.UserID)
	c.Request = c.Request.WithContext(ctx)
	requestctx.SetClaims(c, claims)

	if !claims.UserID.IsZero() {
		requestctx.SetUserID(c, claims.UserID)
	}
	if !claims.LoginIdentityID.IsZero() {
		requestctx.SetLoginIdentityID(c, claims.LoginIdentityID)
	}
	if domain := claims.TenantDomain; domain != "" {
		requestctx.SetTenantID(c, domain)
	}
	if !claims.OrgID.IsZero() {
		requestctx.SetOrgID(c, claims.OrgID.Uint64())
	}
	if claims.TokenID != "" {
		requestctx.SetTokenID(c, claims.TokenID)
	}
}

// extractToken 从请求中提取令牌
// 支持多种方式：Authorization Header, Query Parameter, Cookie
func (m *JWTAuthMiddleware) extractToken(c *gin.Context) (string, string) {
	// 1. 从 Authorization Header 提取
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// 支持 "Bearer <token>" 格式
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1], "header"
		}
		// 也支持直接传递令牌（无 Bearer 前缀）
		return authHeader, "header"
	}

	// 2. 从查询参数提取
	if token := c.Query("token"); token != "" {
		return token, "query"
	}

	// 3. 从 Cookie 提取
	if token, err := c.Cookie("access_token"); err == nil && token != "" {
		return token, "cookie"
	}

	return "", "none"
}

func requestIDFromContext(c *gin.Context) string {
	if rid, ok := requestctx.RequestIDString(c); ok {
		return rid
	}
	if rid := c.GetHeader("X-Request-Id"); rid != "" {
		return rid
	}
	return ""
}
