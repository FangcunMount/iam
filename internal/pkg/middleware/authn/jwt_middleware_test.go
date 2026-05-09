package authn

import (
	"context"
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
		"https://iam.fangcunmount.cn",
		[]string{"qs-api"},
		nil,
		[]string{"pwd"},
		time.Now(),
		time.Now().Add(time.Hour),
	)

	applyVerifiedClaims(c, claims)

	if got := requestctx.TenantIDOrDefault(c); got != "1" {
		t.Fatalf("TenantIDOrDefault() = %q, want %q", got, "1")
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
		requestctx.SetTenantID(c, "1")
		c.Next()
	})

	middleware := NewJWTAuthMiddleware(nil, routeAuthorizationStub{
		rolesByDomain: map[string][]string{
			"1":               {"role:qs:admin"},
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
		requestctx.SetTenantID(c, "1")
		c.Next()
	})

	middleware := NewJWTAuthMiddleware(nil, routeAuthorizationStub{
		rolesByDomain: map[string][]string{
			"1": {"role:qs:admin"},
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
}

func (s routeAuthorizationStub) AuthorizeRoute(_ context.Context, _, _, _, _ string) (bool, error) {
	return true, nil
}

func (s routeAuthorizationStub) DirectRoleKeys(_ context.Context, _, domain string) ([]string, error) {
	return append([]string(nil), s.rolesByDomain[domain]...), nil
}
