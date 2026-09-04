package authn

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/core"
)

// JWTAuthMiddleware JWT 认证中间件
// 使用新的认证模块来验证令牌
type JWTAuthMiddleware struct {
	verifier token.Verifier
}

// NewJWTAuthMiddleware 创建 JWT 认证中间件。
func NewJWTAuthMiddleware(verifier token.Verifier) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{verifier: verifier}
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

		// 验证令牌：资源访问中间件只接受用户 access token。
		resp, err := m.verifier.VerifyToken(c.Request.Context(), token.VerifyTokenRequest{
			AccessToken:        tokenValue,
			AcceptedTokenTypes: []token.TokenType{token.TokenTypeAccess},
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
