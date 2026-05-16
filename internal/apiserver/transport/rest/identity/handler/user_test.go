package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	appuser "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

func TestResolveRolesIncludesPlatformRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v2/identity/me", nil)
	requestctx.SetTenantID(c, "fangcun")

	h := &UserHandler{
		roles: userRoleLookupStub{
			rolesByDomain: map[string][]string{
				"1":               {"role:qs:admin"},
				tenant.PlatformID: {"role:super_admin", "role:qs:admin"},
			},
		},
	}

	got := h.resolveRoles(c, meta.FromUint64(10001))
	want := []string{"qs:admin", "super_admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveRoles() = %#v, want %#v", got, want)
	}
}

func TestNewUserResponseUsesNicknameAndFallsBackToName(t *testing.T) {
	got := newUserResponse(&appuser.UserResult{
		ID:       "10001",
		Name:     "法定名",
		Nickname: "昵称",
	}, nil)
	if got.Nickname != "昵称" {
		t.Fatalf("nickname = %q, want %q", got.Nickname, "昵称")
	}

	got = newUserResponse(&appuser.UserResult{
		ID:   "10001",
		Name: "展示名",
	}, nil)
	if got.Nickname != "展示名" {
		t.Fatalf("fallback nickname = %q, want %q", got.Nickname, "展示名")
	}
}

type userRoleLookupStub struct {
	rolesByDomain map[string][]string
}

func (s userRoleLookupStub) RoleNamesForSubject(_ context.Context, _ subject.Ref, domain string) ([]string, error) {
	return append([]string(nil), s.rolesByDomain[domain]...), nil
}
