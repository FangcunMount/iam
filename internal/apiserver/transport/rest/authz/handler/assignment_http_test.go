package handler

import (
	"context"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

func TestAssignmentHandlerGrantRoleHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{}`, handler.GrantRole, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.grantCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"user-1","role_id":"11"}`, handler.GrantRole, withUser("operator-1"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.grantCalls)
	})

	t.Run("missing user returns token invalid", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"user-1","role_id":"11"}`, handler.GrantRole, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.grantCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &assignmentCommanderFake{
			grantFn: func(context.Context, assignmentDomain.GrantCommand) (*assignmentDomain.Assignment, error) {
				return nil, perrors.WithCode(code.ErrAssignmentAlreadyExists, "exists")
			},
		}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"user-1","role_id":"11"}`, handler.GrantRole, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusConflict, code.ErrAssignmentAlreadyExists)
		require.Len(t, commander.grantCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/grant", `{"subject_type":"user","subject_id":"user-1","role_id":"11"}`, handler.GrantRole, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.grantCalls, 1)
		require.Equal(t, assignmentDomain.GrantCommand{
			SubjectType: assignmentDomain.SubjectTypeUser,
			SubjectID:   "user-1",
			RoleID:      11,
			TenantID:    "tenant-a",
			GrantedBy:   "operator-1",
		}, commander.grantCalls[0])
	})
}

func TestAssignmentHandlerRevokeRoleHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/revoke", `{}`, handler.RevokeRole, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.revokeCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/revoke", `{"subject_type":"user","subject_id":"user-1","role_id":"11"}`, handler.RevokeRole)

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.revokeCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &assignmentCommanderFake{
			revokeFn: func(context.Context, assignmentDomain.RevokeCommand) error {
				return perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/assignments/revoke", `{"subject_type":"user","subject_id":"user-1","role_id":"11"}`, handler.RevokeRole, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, commander.revokeCalls, 1)
	})
}

func TestAssignmentHandlerRevokeRoleByIDHTTPBranches(t *testing.T) {
	t.Run("invalid path id", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/assignments/bad", "", handler.RevokeRoleByID, withPathParam("id", "bad"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, commander.revokeByIDCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &assignmentCommanderFake{}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/assignments/31", "", handler.RevokeRoleByID, withPathParam("id", "31"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.revokeByIDCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &assignmentCommanderFake{
			revokeByIDFn: func(context.Context, assignmentDomain.RevokeByIDCommand) error {
				return perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewAssignmentHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/assignments/31", "", handler.RevokeRoleByID, withPathParam("id", "31"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, commander.revokeByIDCalls, 1)
		require.Equal(t, uint64(31), commander.revokeByIDCalls[0].AssignmentID.Uint64())
		require.Equal(t, "tenant-a", commander.revokeByIDCalls[0].TenantID)
	})
}

func TestAssignmentHandlerListBySubjectHTTPBranches(t *testing.T) {
	t.Run("missing query params returns invalid argument", func(t *testing.T) {
		queryer := &assignmentQueryerFake{}
		handler := NewAssignmentHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject", "", handler.ListAssignmentsBySubject, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.listBySubjectCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		queryer := &assignmentQueryerFake{}
		handler := NewAssignmentHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject?subject_type=user&subject_id=user-1", "", handler.ListAssignmentsBySubject)

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, queryer.listBySubjectCalls)
	})

	t.Run("invalid subject type returns invalid argument", func(t *testing.T) {
		queryer := &assignmentQueryerFake{}
		handler := NewAssignmentHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject?subject_type=service&subject_id=svc-1", "", handler.ListAssignmentsBySubject, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.listBySubjectCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &assignmentQueryerFake{
			listBySubjectFn: func(context.Context, assignmentDomain.ListBySubjectQuery) ([]*assignmentDomain.Assignment, error) {
				return nil, perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewAssignmentHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/assignments/subject?subject_type=user&subject_id=user-1", "", handler.ListAssignmentsBySubject, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, queryer.listBySubjectCalls, 1)
	})
}

func TestAssignmentHandlerListByRoleHTTPBranches(t *testing.T) {
	t.Run("invalid path id", func(t *testing.T) {
		queryer := &assignmentQueryerFake{}
		handler := NewAssignmentHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/bad/assignments", "", handler.ListAssignmentsByRole, withPathParam("id", "bad"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.listByRoleCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		queryer := &assignmentQueryerFake{}
		handler := NewAssignmentHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/11/assignments", "", handler.ListAssignmentsByRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, queryer.listByRoleCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &assignmentQueryerFake{
			listByRoleFn: func(context.Context, assignmentDomain.ListByRoleQuery) ([]*assignmentDomain.Assignment, error) {
				return nil, perrors.WithCode(code.ErrAssignmentNotFound, "missing")
			},
		}
		handler := NewAssignmentHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/11/assignments", "", handler.ListAssignmentsByRole, withPathParam("id", "11"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrAssignmentNotFound)
		require.Len(t, queryer.listByRoleCalls, 1)
		require.Equal(t, assignmentDomain.ListByRoleQuery{RoleID: 11, TenantID: "tenant-a"}, queryer.listByRoleCalls[0])
	})
}
