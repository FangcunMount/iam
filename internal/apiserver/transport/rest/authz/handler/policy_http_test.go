package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	policyApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policylint"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/dto"
	"github.com/stretchr/testify/require"

	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestPolicyHandlerAddPermissionHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{}`, handler.AddPermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.addCalls)
	})

	t.Run("zero role id is rejected before commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"0","resource_id":"21","action":"read"}`, handler.AddPermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.addCalls)
	})

	t.Run("zero resource id is rejected before commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"0","action":"read"}`, handler.AddPermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.addCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.AddPermission, withUser("1001"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.addCalls)
	})

	t.Run("missing user returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.AddPermission, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.addCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &policyCommanderFake{
			addFn: func(context.Context, policyApp.AddPermissionCommand) error {
				return perrors.WithCode(code.ErrPermissionDenied, "denied")
			},
		}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read","scope_type":"origin","scope_value":"1","reason":"test"}`, handler.AddPermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusForbidden, code.ErrPermissionDenied)
		require.Len(t, commander.addCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read","scope_type":"origin","scope_value":"1","reason":"test"}`, handler.AddPermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.addCalls, 1)
		require.Equal(t, meta.FromUint64(11), commander.addCalls[0].RoleID)
		require.Equal(t, uint64(21), commander.addCalls[0].ResourceID.Uint64())
		require.Equal(t, "read", commander.addCalls[0].ActionString())
		require.Equal(t, scope.Scope{Kind: scope.KindOrigin, Value: "1"}, commander.addCalls[0].Scope)
		require.Equal(t, "tenant-a", commander.addCalls[0].TenantIDString())
		require.Equal(t, "1001", commander.addCalls[0].ChangedByString())
		require.Equal(t, "test", commander.addCalls[0].Reason)
	})
}

func TestPolicyHandlerRemovePermissionHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{}`, handler.RemovePermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("zero role id is rejected before commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"0","resource_id":"21","action":"read"}`, handler.RemovePermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.RemovePermission, withUser("1001"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("missing user returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.RemovePermission, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &policyCommanderFake{
			removeFn: func(context.Context, policyApp.RemovePermissionCommand) error {
				return perrors.WithCode(code.ErrPermissionDenied, "denied")
			},
		}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.RemovePermission, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusForbidden, code.ErrPermissionDenied)
		require.Len(t, commander.removeCalls, 1)
	})
}

func TestPolicyHandlerGetPoliciesByRoleHTTPBranches(t *testing.T) {
	t.Run("invalid path id", func(t *testing.T) {
		queryer := &policyQueryerFake{}
		handler := NewPolicyHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/bad/policies", "", handler.GetPoliciesByRole, withPathParam("id", "bad"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrInvalidArgument)
		require.Empty(t, queryer.getPoliciesCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		queryer := &policyQueryerFake{}
		handler := NewPolicyHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/11/policies", "", handler.GetPoliciesByRole, withPathParam("id", "11"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, queryer.getPoliciesCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &policyQueryerFake{
			getPoliciesFn: func(context.Context, policyApp.RolePermissionsQuery) ([]permission.Permission, error) {
				return nil, perrors.WithCode(code.ErrPermissionDenied, "denied")
			},
		}
		handler := NewPolicyHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/roles/11/policies", "", handler.GetPoliciesByRole, withPathParam("id", "11"), withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusForbidden, code.ErrPermissionDenied)
		require.Len(t, queryer.getPoliciesCalls, 1)
	})
}

func TestPolicyHandlerGetCurrentVersionHTTPBranches(t *testing.T) {
	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		queryer := &policyQueryerFake{}
		handler := NewPolicyHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/version", "", handler.GetCurrentVersion)

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, queryer.getCurrentVersionCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		queryer := &policyQueryerFake{
			getCurrentVersionFn: func(context.Context, policyApp.CurrentVersionQuery) (*policyDomain.PolicyVersion, error) {
				return nil, perrors.WithCode(code.ErrPolicyVersionNotFound, "missing")
			},
		}
		handler := NewPolicyHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/version", "", handler.GetCurrentVersion, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusNotFound, code.ErrPolicyVersionNotFound)
		require.Len(t, queryer.getCurrentVersionCalls, 1)
	})

	t.Run("nil version keeps zero version success envelope", func(t *testing.T) {
		queryer := &policyQueryerFake{
			getCurrentVersionFn: func(context.Context, policyApp.CurrentVersionQuery) (*policyDomain.PolicyVersion, error) {
				return nil, nil
			},
		}
		handler := NewPolicyHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/version", "", handler.GetCurrentVersion, withTenant("tenant-a"))

		envelope := requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Contains(t, string(envelope.Data), `"tenant_id":"tenant-a"`)
		require.Contains(t, string(envelope.Data), `"version":0`)
		require.Len(t, queryer.getCurrentVersionCalls, 1)
		require.Equal(t, policyApp.CurrentVersionQuery{TenantID: "tenant-a"}, queryer.getCurrentVersionCalls[0])
	})
}

func TestPolicyHandlerLintPoliciesHTTPBranches(t *testing.T) {
	t.Run("missing linter returns internal error", func(t *testing.T) {
		handler := NewPolicyHandler(nil, nil)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/lint", "", handler.LintPolicies, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusInternalServerError, code.ErrInternalServerError)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		linter := &policyLinterFake{
			lintFn: func(context.Context) (policylint.Report, error) {
				return policylint.Report{}, perrors.WithCode(code.ErrPermissionDenied, "denied")
			},
		}
		handler := NewPolicyHandler(nil, nil, linter)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/lint", "", handler.LintPolicies, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusForbidden, code.ErrPermissionDenied)
		require.Equal(t, 1, linter.calls)
	})

	t.Run("success returns empty report", func(t *testing.T) {
		linter := &policyLinterFake{}
		handler := NewPolicyHandler(nil, nil, linter)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/lint", "", handler.LintPolicies, withTenant("tenant-a"))

		envelope := requireAuthzCode(t, recorder, http.StatusOK, 200)
		var response dto.PolicyLintResponse
		require.NoError(t, json.Unmarshal(envelope.Data, &response))
		require.Empty(t, response.Findings)
		require.Equal(t, 1, linter.calls)
	})

	t.Run("success returns findings contract", func(t *testing.T) {
		linter := &policyLinterFake{
			lintFn: func(context.Context) (policylint.Report, error) {
				return policylint.Report{Findings: []policylint.Finding{
					{
						Code:        policylint.FindingMissingResource,
						RoleName:    "iam:admin",
						TenantID:    "tenant-a",
						ResourceKey: "iam:identity:collection:users",
						Action:      "read",
						Scope:       "all:*",
						Message:     "resource catalog has no matching resource",
					},
					{
						Code:        policylint.FindingUnsupportedAction,
						RoleName:    "qs:admin",
						TenantID:    "tenant-b",
						ResourceKey: "qs:actor:collection:testees",
						Action:      "export",
						Scope:       "origin:1",
						Message:     "resource catalog does not support action",
					},
				}}, nil
			},
		}
		handler := NewPolicyHandler(nil, nil, linter)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/lint", "", handler.LintPolicies, withTenant("tenant-a"))

		envelope := requireAuthzCode(t, recorder, http.StatusOK, 200)
		var response dto.PolicyLintResponse
		require.NoError(t, json.Unmarshal(envelope.Data, &response))
		require.Equal(t, []dto.PolicyLintFindingResponse{
			{
				Code:        "missing_resource",
				RoleName:    "iam:admin",
				TenantID:    "tenant-a",
				ResourceKey: "iam:identity:collection:users",
				Action:      "read",
				Scope:       "all:*",
				Message:     "resource catalog has no matching resource",
			},
			{
				Code:        "unsupported_action",
				RoleName:    "qs:admin",
				TenantID:    "tenant-b",
				ResourceKey: "qs:actor:collection:testees",
				Action:      "export",
				Scope:       "origin:1",
				Message:     "resource catalog does not support action",
			},
		}, response.Findings)
		require.Equal(t, 1, linter.calls)
	})
}
