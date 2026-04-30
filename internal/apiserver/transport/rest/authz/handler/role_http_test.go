package handler

import (
	"context"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	roleApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/role"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func TestRoleHandlerCreateRoleHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &roleCommanderFake{}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/roles", `{}`, handler.CreateRole, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.createCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &roleCommanderFake{}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/roles", `{"name":"admin","display_name":"Admin"}`, handler.CreateRole)

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.createCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &roleCommanderFake{
			createFn: func(context.Context, roleApp.CreateRoleCommand) (*roleDomain.Role, error) {
				return nil, perrors.WithCode(code.ErrRoleAlreadyExists, "role exists")
			},
		}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/roles", `{"name":"admin","display_name":"Admin"}`, handler.CreateRole, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusConflict, code.ErrRoleAlreadyExists)
		require.Len(t, commander.createCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &roleCommanderFake{}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/roles", `{"name":"admin","display_name":"Admin","description":"desc"}`, handler.CreateRole, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.createCalls, 1)
		require.Equal(t, roleApp.CreateRoleCommand{
			Name:        "admin",
			DisplayName: "Admin",
			TenantID:    "tenant-a",
			Description: "desc",
		}, commander.createCalls[0])
	})
}

func TestRoleHandlerUpdateRoleHTTPBranches(t *testing.T) {
	t.Run("invalid path id", func(t *testing.T) {
		commander := &roleCommanderFake{}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/roles/bad", `{"display_name":"Admin"}`, handler.UpdateRole, withPathParam("id", "bad"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, commander.updateCalls)
	})

	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &roleCommanderFake{}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/roles/11", `{"display_name":`, handler.UpdateRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.updateCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &roleCommanderFake{
			updateFn: func(context.Context, roleApp.UpdateRoleCommand) (*roleDomain.Role, error) {
				return nil, perrors.WithCode(code.ErrRoleNotFound, "missing")
			},
		}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/roles/11", `{"display_name":"Admin"}`, handler.UpdateRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrRoleNotFound)
		require.Len(t, commander.updateCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &roleCommanderFake{}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPut, "/roles/11", `{"display_name":"Admin","description":"desc"}`, handler.UpdateRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.updateCalls, 1)
		require.Equal(t, uint64(11), commander.updateCalls[0].ID.Uint64())
		require.NotNil(t, commander.updateCalls[0].DisplayName)
		require.Equal(t, "Admin", *commander.updateCalls[0].DisplayName)
		require.NotNil(t, commander.updateCalls[0].Description)
		require.Equal(t, "desc", *commander.updateCalls[0].Description)
	})
}

func TestRoleHandlerDeleteAndGetHTTPBranches(t *testing.T) {
	t.Run("delete invalid id", func(t *testing.T) {
		commander := &roleCommanderFake{}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/roles/bad", "", handler.DeleteRole, withPathParam("id", "bad"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, commander.deleteCalls)
	})

	t.Run("delete application error", func(t *testing.T) {
		commander := &roleCommanderFake{
			deleteFn: func(context.Context, meta.ID) error {
				return perrors.WithCode(code.ErrRoleNotFound, "missing")
			},
		}
		handler := NewRoleHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/roles/11", "", handler.DeleteRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrRoleNotFound)
		require.Len(t, commander.deleteCalls, 1)
	})

	t.Run("get invalid id", func(t *testing.T) {
		queryer := &roleQueryerFake{}
		handler := NewRoleHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/bad", "", handler.GetRole, withPathParam("id", "bad"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.getCalls)
	})

	t.Run("get application error", func(t *testing.T) {
		queryer := &roleQueryerFake{
			getFn: func(context.Context, meta.ID) (*roleDomain.Role, error) {
				return nil, perrors.WithCode(code.ErrRoleNotFound, "missing")
			},
		}
		handler := NewRoleHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/11", "", handler.GetRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrRoleNotFound)
		require.Len(t, queryer.getCalls, 1)
		require.Equal(t, uint64(11), queryer.getCalls[0].Uint64())
	})
}

func TestRoleHandlerListRolesHTTPBranches(t *testing.T) {
	t.Run("bind error does not call queryer", func(t *testing.T) {
		queryer := &roleQueryerFake{}
		handler := NewRoleHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles?offset=bad", "", handler.ListRoles, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, queryer.listCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		queryer := &roleQueryerFake{}
		handler := NewRoleHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles", "", handler.ListRoles)

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, queryer.listCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &roleQueryerFake{
			listFn: func(context.Context, roleApp.ListRolesQuery) (*roleApp.ListRolesResult, error) {
				return nil, perrors.WithCode(code.ErrInternalServerError, "list failed")
			},
		}
		handler := NewRoleHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles?offset=2&limit=5", "", handler.ListRoles, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusInternalServerError, code.ErrInternalServerError)
		require.Len(t, queryer.listCalls, 1)
	})

	t.Run("success forwards query fields", func(t *testing.T) {
		queryer := &roleQueryerFake{}
		handler := NewRoleHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles?offset=2&limit=5", "", handler.ListRoles, withTenant("tenant-a"))

		envelope := requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Equal(t, int64(1), envelope.Total)
		require.Equal(t, 2, envelope.Offset)
		require.Equal(t, 5, envelope.Limit)
		require.Len(t, queryer.listCalls, 1)
		require.Equal(t, roleApp.ListRolesQuery{TenantID: "tenant-a", Offset: 2, Limit: 5}, queryer.listCalls[0])
	})
}
