package handler

import (
	"context"
	roleapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/testutil"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestRoleDetailsAreTenantScoped(t *testing.T) {
	fixture := testutil.NewFixture(t, nil)
	item, err := role.NewRole("reader", "Reader", "tenant-a")
	require.NoError(t, err)
	require.NoError(t, fixture.Roles.Create(context.Background(), &item))
	for _, tc := range []struct {
		tenant, id string
		status     int
	}{
		{"tenant-a", item.ID.String(), 200}, {"tenant-b", item.ID.String(), 404}, {"platform", item.ID.String(), 404}, {"tenant-a", "987654321", 404},
	} {
		router := gin.New()
		router.Use(func(c *gin.Context) { requestctx.SetTenantID(c, tc.tenant); c.Next() })
		router.GET("/:id", NewRoleHandler(nil, roleapp.NewRoleQueryService(fixture.Roles)).GetRole)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/"+tc.id, nil))
		require.Equal(t, tc.status, response.Code, response.Body.String())
	}
}
