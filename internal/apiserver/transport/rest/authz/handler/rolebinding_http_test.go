package handler

import (
	"context"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	bindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestRoleBindingHandlerGrantRoleHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{}`, handler.GrantRoleBinding, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.grantCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"1","role_id":"11"}`, handler.GrantRoleBinding, withUser("1001"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.grantCalls)
	})

	t.Run("missing user returns token invalid", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"1","role_id":"11"}`, handler.GrantRoleBinding, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.grantCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &bindingCommanderFake{
			grantFn: func(context.Context, bindingApp.GrantCommand) (*bindingDomain.Binding, error) {
				return nil, perrors.WithCode(code.ErrAssignmentAlreadyExists, "exists")
			},
		}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"1","role_id":"11"}`, handler.GrantRoleBinding, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusConflict, code.ErrAssignmentAlreadyExists)
		require.Len(t, commander.grantCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"1","role_id":"11"}`, handler.GrantRoleBinding, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.grantCalls, 1)
		require.Equal(t, bindingApp.GrantCommand{
			SubjectType: bindingDomain.SubjectTypeUser,
			SubjectID:   meta.FromUint64(1),
			RoleID:      meta.FromUint64(11),
			TenantID:    "tenant-a",
			GrantedBy:   "1001",
		}, commander.grantCalls[0])
	})
}

func TestRoleBindingHandlerRevokeRoleHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/revoke", `{}`, handler.RevokeRoleBinding, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.revokeCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/revoke", `{"subject_type":"user","subject_id":"1","role_id":"11"}`, handler.RevokeRoleBinding)

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.revokeCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &bindingCommanderFake{
			revokeFn: func(context.Context, bindingApp.RevokeCommand) error {
				return perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/revoke", `{"subject_type":"user","subject_id":"1","role_id":"11"}`, handler.RevokeRoleBinding, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, commander.revokeCalls, 1)
	})

	t.Run("success forwards audit fields", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/revoke", `{"subject_type":"user","subject_id":"1","role_id":"11","reason":"manual revoke"}`, handler.RevokeRoleBinding, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.revokeCalls, 1)
		require.Equal(t, bindingApp.RevokeCommand{
			SubjectType: bindingDomain.SubjectTypeUser,
			SubjectID:   meta.FromUint64(1),
			RoleID:      meta.FromUint64(11),
			TenantID:    "tenant-a",
			ChangedBy:   "1001",
			Reason:      "manual revoke",
		}, commander.revokeCalls[0])
	})
}

func TestRoleBindingHandlerRevokeRoleBindingByIDHTTPBranches(t *testing.T) {
	t.Run("invalid path id", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/assignments/bad", "", handler.RevokeRoleBindingByID, withPathParam("id", "bad"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, commander.revokeByIDCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &bindingCommanderFake{}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/assignments/31", "", handler.RevokeRoleBindingByID, withPathParam("id", "31"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.revokeByIDCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &bindingCommanderFake{
			revokeByIDFn: func(context.Context, bindingApp.RevokeByIDCommand) error {
				return perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewRoleBindingHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/assignments/31", "", handler.RevokeRoleBindingByID, withPathParam("id", "31"), withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, commander.revokeByIDCalls, 1)
		require.Equal(t, uint64(31), commander.revokeByIDCalls[0].BindingID.Uint64())
		require.Equal(t, "tenant-a", commander.revokeByIDCalls[0].TenantID)
		require.Equal(t, "1001", commander.revokeByIDCalls[0].ChangedBy)
	})
}

func TestRoleBindingHandlerListBySubjectHTTPBranches(t *testing.T) {
	t.Run("missing query params returns invalid argument", func(t *testing.T) {
		queryer := &bindingQueryerFake{}
		handler := NewRoleBindingHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject", "", handler.ListRoleBindingsBySubject, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.listBySubjectCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		queryer := &bindingQueryerFake{}
		handler := NewRoleBindingHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject?subject_type=user&subject_id=1", "", handler.ListRoleBindingsBySubject)

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, queryer.listBySubjectCalls)
	})

	t.Run("invalid subject type returns invalid argument", func(t *testing.T) {
		queryer := &bindingQueryerFake{}
		handler := NewRoleBindingHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject?subject_type=service&subject_id=svc-1", "", handler.ListRoleBindingsBySubject, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.listBySubjectCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &bindingQueryerFake{
			listBySubjectFn: func(context.Context, bindingApp.ListBySubjectQuery) ([]*bindingDomain.Binding, error) {
				return nil, perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewRoleBindingHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject?subject_type=user&subject_id=1", "", handler.ListRoleBindingsBySubject, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, queryer.listBySubjectCalls, 1)
	})
}

func TestRoleBindingHandlerListByRoleHTTPBranches(t *testing.T) {
	t.Run("invalid path id", func(t *testing.T) {
		queryer := &bindingQueryerFake{}
		handler := NewRoleBindingHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/bad/assignments", "", handler.ListRoleBindingsByRole, withPathParam("id", "bad"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.listByRoleCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		queryer := &bindingQueryerFake{}
		handler := NewRoleBindingHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/11/assignments", "", handler.ListRoleBindingsByRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, queryer.listByRoleCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &bindingQueryerFake{
			listByRoleFn: func(context.Context, bindingApp.ListByRoleQuery) ([]*bindingDomain.Binding, error) {
				return nil, perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewRoleBindingHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/11/assignments", "", handler.ListRoleBindingsByRole, withPathParam("id", "11"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, queryer.listByRoleCalls, 1)
		require.Equal(t, bindingApp.ListByRoleQuery{RoleID: meta.FromUint64(11), TenantID: "tenant-a"}, queryer.listByRoleCalls[0])
	})
}
