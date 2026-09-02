package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpdateResourceDisplayNameJSONStates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name            string
		body            string
		wantCalled      bool
		wantDisplayName *string
	}{
		{name: "omitted keeps current value", body: `{}`, wantCalled: true},
		{name: "explicit empty is rejected", body: `{"display_name":"   "}`},
		{name: "explicit value is forwarded", body: `{"display_name":" Updated "}`, wantCalled: true, wantDisplayName: stringPointer(" Updated ")},
		{name: "explicit empty actions are rejected", body: `{"actions":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := &recordingResourceCatalog{}
			handler := NewResourceHandler(catalog, nil)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				requestctx.SetTenantID(c, "tenant-a")
				requestctx.SetUserID(c, meta.FromUint64(100))
				c.Next()
			})
			router.PUT("/:id", handler.UpdateResource)

			request := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if !tt.wantCalled {
				require.Empty(t, catalog.updates)
				require.NotEqual(t, http.StatusOK, response.Code)
				return
			}
			require.Equal(t, http.StatusOK, response.Code)
			require.Len(t, catalog.updates, 1)
			cmd := catalog.updates[0]
			require.Equal(t, tt.wantDisplayName, cmd.DisplayName)
			require.Equal(t, "tenant-a", cmd.TenantID)
			require.Equal(t, "100", cmd.ChangedBy)
		})
	}
}

type recordingResourceCatalog struct {
	updates []resourceApp.UpdateResourceCommand
}

func (c *recordingResourceCatalog) CreateResource(context.Context, resourceApp.CreateResourceCommand) (*resourceDomain.Resource, error) {
	return nil, nil
}

func (c *recordingResourceCatalog) UpdateResource(_ context.Context, cmd resourceApp.UpdateResourceCommand) (*resourceDomain.Resource, error) {
	c.updates = append(c.updates, cmd)
	resource, err := resourceDomain.NewResource(
		"qs:evaluation:collection:assessments",
		[]string{"read"},
		resourceDomain.WithID(cmd.ID),
		resourceDomain.WithDisplayName("Assessments"),
	)
	if err != nil {
		return nil, err
	}
	if cmd.DisplayName != nil {
		if err := resource.Rename(*cmd.DisplayName); err != nil {
			return nil, err
		}
	}
	return &resource, nil
}

func (c *recordingResourceCatalog) DeleteResource(context.Context, resourceApp.DeleteResourceCommand) error {
	return nil
}

func stringPointer(value string) *string {
	return &value
}
