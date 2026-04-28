package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
	"github.com/FangcunMount/iam/pkg/tenant"
)

func TestResolveRolesIncludesPlatformRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	c.Set(authn.ContextKeyTenantID, "1")

	h := &UserHandler{
		casbin: userRoleLookupStub{
			rolesByDomain: map[string][]string{
				"1":               {"role:qs:admin"},
				tenant.PlatformID: {"role:super_admin", "role:qs:admin"},
			},
		},
	}

	got := h.resolveRoles(c, "10001")
	want := []string{"qs:admin", "super_admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveRoles() = %#v, want %#v", got, want)
	}
}

type userRoleLookupStub struct {
	rolesByDomain map[string][]string
}

func (s userRoleLookupStub) Enforce(_ context.Context, _, _, _, _ string) (bool, error) {
	return true, nil
}

func (s userRoleLookupStub) GetRolesForUser(_ context.Context, _, domain string) ([]string, error) {
	return append([]string(nil), s.rolesByDomain[domain]...), nil
}
