package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
	"github.com/gin-gonic/gin"
)

type routePermissionCheckerStub struct {
	allowedByTenant map[string]bool
	errorsByTenant  map[string]error
}

func (s routePermissionCheckerStub) CheckRoutePermission(_ context.Context, _, tenantID, _, _ string) (bool, error) {
	return s.allowedByTenant[tenantID], s.errorsByTenant[tenantID]
}

func TestRequirePermissionOrGlobal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		checker    routePermissionCheckerStub
		withUser   bool
		wantStatus int
	}{
		{name: "domain permission", checker: routePermissionCheckerStub{allowedByTenant: map[string]bool{tenant.DefaultID: true}}, withUser: true, wantStatus: http.StatusNoContent},
		{name: "platform permission", checker: routePermissionCheckerStub{allowedByTenant: map[string]bool{tenant.PlatformID: true}}, withUser: true, wantStatus: http.StatusNoContent},
		{name: "both domains deny", checker: routePermissionCheckerStub{}, withUser: true, wantStatus: http.StatusForbidden},
		{name: "domain failure and platform deny", checker: routePermissionCheckerStub{errorsByTenant: map[string]error{tenant.DefaultID: errors.New("domain sentinel")}}, withUser: true, wantStatus: http.StatusInternalServerError},
		{name: "domain failure but platform allows", checker: routePermissionCheckerStub{allowedByTenant: map[string]bool{tenant.PlatformID: true}, errorsByTenant: map[string]error{tenant.DefaultID: errors.New("domain sentinel")}}, withUser: true, wantStatus: http.StatusNoContent},
		{name: "platform failure and domain deny", checker: routePermissionCheckerStub{errorsByTenant: map[string]error{tenant.PlatformID: errors.New("platform sentinel")}}, withUser: true, wantStatus: http.StatusInternalServerError},
		{name: "expired policy", checker: routePermissionCheckerStub{errorsByTenant: map[string]error{tenant.DefaultID: perrors.WithCode(code.ErrAuthorizationPolicyUnavailable, "expired")}, allowedByTenant: map[string]bool{tenant.PlatformID: true}}, withUser: true, wantStatus: http.StatusServiceUnavailable},
		{name: "missing principal", checker: routePermissionCheckerStub{}, wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			if tc.withUser {
				engine.Use(func(c *gin.Context) {
					requestctx.SetUserID(c, meta.FromUint64(10001))
					requestctx.SetTenantID(c, tenant.DefaultID)
					c.Next()
				})
			}
			middleware := NewMiddleware(tc.checker)
			engine.GET("/protected",
				middleware.RequirePermissionOrGlobal("iam:authz:collection:roles", "read"),
				func(c *gin.Context) { c.Status(http.StatusNoContent) },
			)

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
		})
	}
}
