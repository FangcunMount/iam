package handler

import (
	"context"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	resourceApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/resource"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

func TestResourceHandlerCreateResourceHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &resourceCommanderFake{}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/resources", `{}`, handler.CreateResource)

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.createCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &resourceCommanderFake{
			createFn: func(context.Context, resourceApp.CreateResourceCommand) (*resourceDomain.Resource, error) {
				return nil, perrors.WithCode(code.ErrResourceAlreadyExists, "exists")
			},
		}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/resources", `{"key":"scale:form:*","display_name":"Form","app_name":"scale","domain":"form","type":"*","actions":["read"]}`, handler.CreateResource)

		requireAuthzCode(t, recorder, http.StatusConflict, code.ErrResourceAlreadyExists)
		require.Len(t, commander.createCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &resourceCommanderFake{}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/resources", `{"key":"scale:form:*","display_name":"Form","app_name":"scale","domain":"form","type":"*","actions":["read","write"],"description":"desc"}`, handler.CreateResource)

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.createCalls, 1)
		require.Equal(t, resourceApp.CreateResourceCommand{
			Key:         "scale:form:*",
			DisplayName: "Form",
			AppName:     "scale",
			Domain:      "form",
			Type:        "*",
			Actions:     []string{"read", "write"},
			Description: "desc",
		}, commander.createCalls[0])
	})
}

func TestResourceHandlerUpdateResourceHTTPBranches(t *testing.T) {
	t.Run("invalid path id", func(t *testing.T) {
		commander := &resourceCommanderFake{}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/resources/bad", `{"display_name":"Form","actions":["read"]}`, handler.UpdateResource, withPathParam("id", "bad"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, commander.updateCalls)
	})

	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &resourceCommanderFake{}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/resources/21", `{"actions":[]}`, handler.UpdateResource, withPathParam("id", "21"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.updateCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &resourceCommanderFake{
			updateFn: func(context.Context, resourceApp.UpdateResourceCommand) (*resourceDomain.Resource, error) {
				return nil, perrors.WithCode(code.ErrResourceNotFound, "missing")
			},
		}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/resources/21", `{"display_name":"Form","actions":["read"]}`, handler.UpdateResource, withPathParam("id", "21"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrResourceNotFound)
		require.Len(t, commander.updateCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &resourceCommanderFake{}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/resources/21", `{"display_name":"Form","actions":["read"],"description":"desc"}`, handler.UpdateResource, withPathParam("id", "21"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.updateCalls, 1)
		require.Equal(t, uint64(21), commander.updateCalls[0].ID.Uint64())
		require.NotNil(t, commander.updateCalls[0].DisplayName)
		require.Equal(t, "Form", *commander.updateCalls[0].DisplayName)
		require.Equal(t, []string{"read"}, commander.updateCalls[0].Actions)
		require.NotNil(t, commander.updateCalls[0].Description)
		require.Equal(t, "desc", *commander.updateCalls[0].Description)
	})
}

func TestResourceHandlerReadAndDeleteHTTPBranches(t *testing.T) {
	t.Run("delete invalid id", func(t *testing.T) {
		commander := &resourceCommanderFake{}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/resources/bad", "", handler.DeleteResource, withPathParam("id", "bad"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, commander.deleteCalls)
	})

	t.Run("delete application error", func(t *testing.T) {
		commander := &resourceCommanderFake{
			deleteFn: func(context.Context, resourceDomain.ResourceID) error {
				return perrors.WithCode(code.ErrResourceNotFound, "missing")
			},
		}
		handler := NewResourceHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/resources/21", "", handler.DeleteResource, withPathParam("id", "21"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrResourceNotFound)
		require.Len(t, commander.deleteCalls, 1)
	})

	t.Run("get invalid id", func(t *testing.T) {
		queryer := &resourceQueryerFake{}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/resources/bad", "", handler.GetResource, withPathParam("id", "bad"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.getByIDCalls)
	})

	t.Run("get application error", func(t *testing.T) {
		queryer := &resourceQueryerFake{
			getByIDFn: func(context.Context, resourceDomain.ResourceID) (*resourceDomain.Resource, error) {
				return nil, perrors.WithCode(code.ErrResourceNotFound, "missing")
			},
		}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/resources/21", "", handler.GetResource, withPathParam("id", "21"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrResourceNotFound)
		require.Len(t, queryer.getByIDCalls, 1)
	})

	t.Run("get by key application error", func(t *testing.T) {
		queryer := &resourceQueryerFake{
			getByKeyFn: func(context.Context, string) (*resourceDomain.Resource, error) {
				return nil, perrors.WithCode(code.ErrResourceNotFound, "missing")
			},
		}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/resources/key/scale:form:*", "", handler.GetResourceByKey, withPathParam("key", "scale:form:*"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrResourceNotFound)
		require.Equal(t, []string{"scale:form:*"}, queryer.getByKeyCalls)
	})
}

func TestResourceHandlerListResourcesHTTPBranches(t *testing.T) {
	t.Run("non numeric pagination keeps current default behavior", func(t *testing.T) {
		queryer := &resourceQueryerFake{}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/resources?offset=bad&limit=bad&app_name=scale&domain=form&type=*", "", handler.ListResources)

		envelope := requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Equal(t, 0, envelope.Offset)
		require.Equal(t, 0, envelope.Limit)
		require.Len(t, queryer.listCalls, 1)
		require.Equal(t, resourceApp.ListResourcesQuery{
			AppName: "scale",
			Domain:  "form",
			Type:    "*",
			Offset:  0,
			Limit:   0,
		}, queryer.listCalls[0])
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &resourceQueryerFake{
			listFn: func(context.Context, resourceApp.ListResourcesQuery) (*resourceApp.ListResourcesResult, error) {
				return nil, perrors.WithCode(code.ErrInternalServerError, "list failed")
			},
		}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/resources?offset=2&limit=5", "", handler.ListResources)

		requireAuthzCode(t, recorder, http.StatusInternalServerError, code.ErrInternalServerError)
		require.Len(t, queryer.listCalls, 1)
	})
}

func TestResourceHandlerValidateActionHTTPBranches(t *testing.T) {
	t.Run("bind error does not call queryer", func(t *testing.T) {
		queryer := &resourceQueryerFake{}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodPost, "/resources/validate-action", `{}`, handler.ValidateAction)

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, queryer.validateCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &resourceQueryerFake{
			validateFn: func(context.Context, string, string) (bool, error) {
				return false, perrors.WithCode(code.ErrInvalidAction, "invalid")
			},
		}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodPost, "/resources/validate-action", `{"resource_key":"scale:form:*","action":"delete"}`, handler.ValidateAction)

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidAction)
		require.Len(t, queryer.validateCalls, 1)
	})

	t.Run("success forwards validation fields", func(t *testing.T) {
		queryer := &resourceQueryerFake{}
		handler := NewResourceHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodPost, "/resources/validate-action", `{"resource_key":"scale:form:*","action":"read"}`, handler.ValidateAction)

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, queryer.validateCalls, 1)
		require.Equal(t, "scale:form:*", queryer.validateCalls[0].resourceKey)
		require.Equal(t, "read", queryer.validateCalls[0].action)
	})
}
