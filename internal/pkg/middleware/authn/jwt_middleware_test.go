package authn

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
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

func TestRequirePlatformAdminAllowsSuperAdminFromPlatformDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		requestctx.SetUserID(c, meta.FromUint64(10001))
		requestctx.SetTenantID(c, tenant.DefaultID)
		c.Next()
	})

	middleware := NewJWTAuthMiddleware(nil, routeAuthorizationStub{
		rolesByDomain: map[string][]string{
			tenant.DefaultID:  {"role:qs:admin"},
			tenant.PlatformID: {"role:super_admin"},
		},
	})

	engine.GET("/protected", middleware.RequirePlatformAdmin(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}

func TestRequirePlatformAdminRejectsTenantOnlyRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		requestctx.SetUserID(c, meta.FromUint64(10001))
		requestctx.SetTenantID(c, tenant.DefaultID)
		c.Next()
	})

	middleware := NewJWTAuthMiddleware(nil, routeAuthorizationStub{
		rolesByDomain: map[string][]string{
			tenant.DefaultID: {"role:qs:admin"},
		},
	})

	engine.GET("/protected", middleware.RequirePlatformAdmin(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestNormalizeRoleName(t *testing.T) {
	got := []string{
		NormalizeRoleName("role:super_admin"),
		NormalizeRoleName(" iam:admin "),
	}
	want := []string{"super_admin", "iam:admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeRoleName() = %#v, want %#v", got, want)
	}
}

type routeAuthorizationStub struct {
	rolesByDomain map[string][]string
	allowed       bool
	authorizeErr  error
	rolesErr      error
}

func (s routeAuthorizationStub) AuthorizeRoute(_ context.Context, _, _, _, _ string) (bool, error) {
	return s.allowed, s.authorizeErr
}

func (s routeAuthorizationStub) DirectRoleKeys(_ context.Context, _, domain string) ([]string, error) {
	if s.rolesErr != nil {
		return nil, s.rolesErr
	}
	return append([]string(nil), s.rolesByDomain[domain]...), nil
}

func TestRequirePermissionOrPlatformAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		runtime    routeAuthorizationStub
		withUser   bool
		wantStatus int
	}{
		{
			name:       "exact permission",
			runtime:    routeAuthorizationStub{allowed: true},
			withUser:   true,
			wantStatus: http.StatusNoContent,
		},
		{
			name: "platform administrator",
			runtime: routeAuthorizationStub{rolesByDomain: map[string][]string{
				tenant.PlatformID: {"role:super_admin"},
			}},
			withUser:   true,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "neither permission nor administrator",
			runtime:    routeAuthorizationStub{},
			withUser:   true,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "permission backend failure",
			runtime:    routeAuthorizationStub{authorizeErr: errors.New("permission sentinel")},
			withUser:   true,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "role backend failure",
			runtime:    routeAuthorizationStub{rolesErr: errors.New("role sentinel")},
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
				middleware.RequirePermissionOrPlatformAdmin("iam:authz:collection:roles", "read"),
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
