// Package authz enforces authorization decisions at the HTTP boundary.
package authz

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/core"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// RoutePermissionChecker answers a route-level permission question without
// controlling the HTTP request lifecycle.
type RoutePermissionChecker interface {
	CheckRoutePermission(ctx context.Context, subjectKey, tenantID, resourceKey, action string) (bool, error)
}

// Middleware turns authorization decisions into HTTP allow/deny execution.
type Middleware struct {
	checker RoutePermissionChecker
}

func NewMiddleware(checker RoutePermissionChecker) *Middleware {
	return &Middleware{checker: checker}
}

func (m *Middleware) Available() bool {
	return m != nil && m.checker != nil
}

// RequirePermission enforces a permission in the request tenant.
// It must run after the AuthN middleware has established the principal.
func (m *Middleware) RequirePermission(resourceKey, action string) gin.HandlerFunc {
	return m.requirePermission(resourceKey, action, "")
}

func (m *Middleware) RequirePlatformPermission(resourceKey, action string) gin.HandlerFunc {
	return m.requirePermission(resourceKey, action, tenant.PlatformID)
}

func (m *Middleware) requirePermission(resourceKey, action, fixedTenant string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.Available() {
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
		subjectKey := "user:" + userID.String()
		tenantID := requestctx.TenantIDOrDefault(c)
		if fixedTenant != "" {
			tenantID = fixedTenant
		}
		allowed, err := m.checker.CheckRoutePermission(c.Request.Context(), subjectKey, tenantID, resourceKey, action)
		if err != nil {
			log.Errorw("route authorization failed", "sub", subjectKey, "tenant_id", tenantID)
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

// RequirePermissionOrGlobal checks the request tenant and then the platform
// tenant. Both checks still require a matching PermissionGrant.
func (m *Middleware) RequirePermissionOrGlobal(resourceKey, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.Available() {
			recordHTTPAuthorization(resourceKey, action, "error")
			core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization engine not configured"), nil)
			c.Abort()
			return
		}
		userID, ok := requestctx.UserID(c)
		if !ok {
			recordHTTPAuthorization(resourceKey, action, "unauthenticated")
			core.WriteResponse(c, errors.WithCode(code.ErrTokenInvalid, "Not authenticated"), nil)
			c.Abort()
			return
		}

		subjectKey := "user:" + userID.String()
		tenantID := requestctx.TenantIDOrDefault(c)
		allowed, tenantErr := m.checker.CheckRoutePermission(
			c.Request.Context(), subjectKey, tenantID, resourceKey, action,
		)
		if allowed {
			recordHTTPAuthorization(resourceKey, action, "domain_permission")
			c.Next()
			return
		}

		var globalErr error
		if tenantID != tenant.PlatformID {
			var globalAllowed bool
			globalAllowed, globalErr = m.checker.CheckRoutePermission(
				c.Request.Context(), subjectKey, tenant.PlatformID, resourceKey, action,
			)
			if globalAllowed {
				recordHTTPAuthorization(resourceKey, action, "global_permission")
				c.Next()
				return
			}
		}
		if tenantErr != nil || globalErr != nil {
			recordHTTPAuthorization(resourceKey, action, "error")
			log.Errorw("route authorization check failed", "resource", resourceKey, "action", action)
			core.WriteResponse(c, errors.WithCode(code.ErrInternalServerError, "Authorization check failed"), nil)
			c.Abort()
			return
		}

		recordHTTPAuthorization(resourceKey, action, "denied")
		core.WriteResponse(c, errors.WithCode(code.ErrPermissionDenied, "Forbidden"), nil)
		c.Abort()
	}
}
