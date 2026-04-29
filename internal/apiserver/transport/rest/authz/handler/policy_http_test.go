package handler

import (
	"context"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

func TestPolicyHandlerAddPolicyRuleHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{}`, handler.AddPolicyRule, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.addCalls)
	})

	t.Run("zero role id is rejected before commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"0","resource_id":"21","action":"read"}`, handler.AddPolicyRule, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.addCalls)
	})

	t.Run("zero resource id is rejected before commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"0","action":"read"}`, handler.AddPolicyRule, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.addCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.AddPolicyRule, withUser("operator-1"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.addCalls)
	})

	t.Run("missing user returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.AddPolicyRule, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.addCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &policyCommanderFake{
			addFn: func(context.Context, policyDomain.AddPolicyRuleCommand) error {
				return perrors.WithCode(code.ErrPermissionDenied, "denied")
			},
		}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read","reason":"test"}`, handler.AddPolicyRule, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusForbidden, code.ErrPermissionDenied)
		require.Len(t, commander.addCalls, 1)
	})

	t.Run("success forwards command fields", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/policies", `{"role_id":"11","resource_id":"21","action":"read","reason":"test"}`, handler.AddPolicyRule, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, commander.addCalls, 1)
		require.Equal(t, uint64(11), commander.addCalls[0].RoleID)
		require.Equal(t, uint64(21), commander.addCalls[0].ResourceID.Uint64())
		require.Equal(t, "read", commander.addCalls[0].Action)
		require.Equal(t, "tenant-a", commander.addCalls[0].TenantID)
		require.Equal(t, "operator-1", commander.addCalls[0].ChangedBy)
		require.Equal(t, "test", commander.addCalls[0].Reason)
	})
}

func TestPolicyHandlerRemovePolicyRuleHTTPBranches(t *testing.T) {
	t.Run("bind error does not call commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{}`, handler.RemovePolicyRule, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("zero role id is rejected before commander", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"0","resource_id":"21","action":"read"}`, handler.RemovePolicyRule, withTenantUser("tenant-a", "operator-1"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.RemovePolicyRule, withUser("operator-1"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("missing user returns token invalid", func(t *testing.T) {
		commander := &policyCommanderFake{}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.RemovePolicyRule, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, commander.removeCalls)
	})

	t.Run("application error is propagated", func(t *testing.T) {
		commander := &policyCommanderFake{
			removeFn: func(context.Context, policyDomain.RemovePolicyRuleCommand) error {
				return perrors.WithCode(code.ErrPermissionDenied, "denied")
			},
		}
		handler := NewPolicyHandler(commander, nil)

		recorder, _ := performAuthzRequest(http.MethodDelete, "/policies", `{"role_id":"11","resource_id":"21","action":"read"}`, handler.RemovePolicyRule, withTenantUser("tenant-a", "operator-1"))

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
			getPoliciesFn: func(context.Context, policyDomain.GetPoliciesByRoleQuery) ([]policyDomain.PolicyRule, error) {
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
			getCurrentVersionFn: func(context.Context, policyDomain.GetCurrentVersionQuery) (*policyDomain.PolicyVersion, error) {
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
			getCurrentVersionFn: func(context.Context, policyDomain.GetCurrentVersionQuery) (*policyDomain.PolicyVersion, error) {
				return nil, nil
			},
		}
		handler := NewPolicyHandler(nil, queryer)

		recorder, _ := performAuthzRequest(http.MethodGet, "/policies/version", "", handler.GetCurrentVersion, withTenant("tenant-a"))

		envelope := requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Contains(t, string(envelope.Data), `"tenant_id":"tenant-a"`)
		require.Contains(t, string(envelope.Data), `"version":0`)
		require.Len(t, queryer.getCurrentVersionCalls, 1)
		require.Equal(t, policyDomain.GetCurrentVersionQuery{TenantID: "tenant-a"}, queryer.getCurrentVersionCalls[0])
	})
}
