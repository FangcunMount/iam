package authn

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

func TestApplyVerifiedClaimsSetsTenantIDForRoleResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/identity/me", nil)

	claims := tokenapp.NewTokenClaims(
		tokenapp.TokenTypeAccess,
		"token-1",
		"user:110001",
		"sid-1",
		meta.ID(110001),
		meta.ID(613486856213901870),
		meta.ID(1),
		tenant.DefaultID,
		"https://iam.fangcunmount.cn",
		[]string{"qs-api"},
		nil,
		[]string{"pwd"},
		time.Now(),
		time.Now().Add(time.Hour),
	)

	applyVerifiedClaims(c, claims)

	if got := requestctx.TenantIDOrDefault(c); got != tenant.DefaultID {
		t.Fatalf("TenantIDOrDefault() = %q, want %q", got, tenant.DefaultID)
	}
	if got, exists := c.Get(requestctx.KeyUserID); !exists || got != meta.ID(110001) {
		t.Fatalf("gin user_id = %v exists=%v, want %v", got, exists, meta.ID(110001))
	}
	if got, exists := c.Get(requestctx.KeyLoginIdentityID); !exists || got != meta.ID(613486856213901870) {
		t.Fatalf("gin login_identity_id = %v exists=%v, want %v", got, exists, meta.ID(613486856213901870))
	}
	if got, exists := c.Get(requestctx.KeyTokenID); !exists || got != "token-1" {
		t.Fatalf("gin token_id = %v exists=%v, want %q", got, exists, "token-1")
	}
}

func TestNormalizeRoleName(t *testing.T) {
	if got := NormalizeRoleName("role:qs:evaluator"); got != "qs:evaluator" {
		t.Fatalf("NormalizeRoleName() = %q, want %q", got, "qs:evaluator")
	}
}

type routeAuthorizationStub struct {
	allowedByDomain map[string]bool
	errorsByDomain  map[string]error
}

func (s routeAuthorizationStub) AuthorizeRoute(_ context.Context, _, domain, _, _ string) (bool, error) {
	return s.allowedByDomain[domain], s.errorsByDomain[domain]
}

func TestRequirePermissionOrGlobal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		runtime    routeAuthorizationStub
		withUser   bool
		wantStatus int
	}{
		{
			name:       "domain permission",
			runtime:    routeAuthorizationStub{allowedByDomain: map[string]bool{tenant.DefaultID: true}},
			withUser:   true,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "platform permission",
			runtime:    routeAuthorizationStub{allowedByDomain: map[string]bool{tenant.PlatformID: true}},
			withUser:   true,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "both domains deny",
			runtime:    routeAuthorizationStub{},
			withUser:   true,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "domain failure and platform deny",
			runtime: routeAuthorizationStub{errorsByDomain: map[string]error{
				tenant.DefaultID: errors.New("domain sentinel"),
			}},
			withUser:   true,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "domain failure but platform allows",
			runtime: routeAuthorizationStub{
				allowedByDomain: map[string]bool{tenant.PlatformID: true},
				errorsByDomain:  map[string]error{tenant.DefaultID: errors.New("domain sentinel")},
			},
			withUser:   true,
			wantStatus: http.StatusNoContent,
		},
		{
			name: "platform failure and domain deny",
			runtime: routeAuthorizationStub{errorsByDomain: map[string]error{
				tenant.PlatformID: errors.New("platform sentinel"),
			}},
			withUser:   true,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "missing principal",
			runtime:    routeAuthorizationStub{},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			if tt.withUser {
				engine.Use(func(c *gin.Context) {
					requestctx.SetUserID(c, meta.FromUint64(10001))
					requestctx.SetTenantID(c, tenant.DefaultID)
					c.Next()
				})
			}
			middleware := NewJWTAuthMiddleware(nil, tt.runtime)
			engine.GET("/protected",
				middleware.RequirePermissionOrGlobal("iam:authz:collection:roles", "read"),
				func(c *gin.Context) { c.Status(http.StatusNoContent) },
			)

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}
