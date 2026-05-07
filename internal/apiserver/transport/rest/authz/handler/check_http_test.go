package handler

import (
	"context"
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

func TestCheckHandlerHTTPBranches(t *testing.T) {
	t.Run("nil casbin returns internal server error", func(t *testing.T) {
		handler := NewCheckHandler(nil)

		recorder, _ := performAuthzRequest(http.MethodPost, "/check", `{"object":"scale:form:*","action":"read"}`, handler.Check, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusInternalServerError, code.ErrInternalServerError)
	})

	t.Run("bind error does not call casbin", func(t *testing.T) {
		casbin := &casbinFake{}
		handler := NewCheckHandler(casbin)

		recorder, _ := performAuthzRequest(http.MethodPost, "/check", `{}`, handler.Check, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusBadRequest, code.ErrBind)
		require.Empty(t, casbin.enforceCalls)
	})

	t.Run("missing subject returns current authorization error", func(t *testing.T) {
		casbin := &casbinFake{}
		handler := NewCheckHandler(casbin)

		recorder, _ := performAuthzRequest(http.MethodPost, "/check", `{"object":"scale:form:*","action":"read"}`, handler.Check, withTenant("tenant-a"))

		requireAuthzCode(t, recorder, http.StatusForbidden, code.ErrUnauthorized)
		require.Empty(t, casbin.enforceCalls)
	})

	t.Run("missing tenant returns token invalid", func(t *testing.T) {
		casbin := &casbinFake{}
		handler := NewCheckHandler(casbin)

		recorder, _ := performAuthzRequest(http.MethodPost, "/check", `{"object":"scale:form:*","action":"read"}`, handler.Check, withUser("1001"))

		requireAuthzCode(t, recorder, http.StatusUnauthorized, code.ErrTokenInvalid)
		require.Empty(t, casbin.enforceCalls)
	})

	t.Run("explicit subject is used when present", func(t *testing.T) {
		casbin := &casbinFake{}
		handler := NewCheckHandler(casbin)

		recorder, _ := performAuthzRequest(http.MethodPost, "/check", `{"subject_type":"user","subject_id":"2","object":"scale:form:*","action":"read","scope_type":"origin","scope_value":"1"}`, handler.Check, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, casbin.enforceCalls, 1)
		require.Equal(t, "user:2", casbin.enforceCalls[0].sub)
		require.Equal(t, "tenant-a", casbin.enforceCalls[0].dom)
		require.Equal(t, "scale:form:*", casbin.enforceCalls[0].obj)
		require.Equal(t, "read", casbin.enforceCalls[0].act)
		require.Equal(t, authzDomain.Scope{Kind: authzDomain.ScopeKindOrigin, Value: "1"}, casbin.enforceCalls[0].scope)
	})

	t.Run("current user fallback is used when subject is omitted", func(t *testing.T) {
		casbin := &casbinFake{}
		handler := NewCheckHandler(casbin)

		recorder, _ := performAuthzRequest(http.MethodPost, "/check", `{"object":"scale:form:*","action":"read"}`, handler.Check, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusOK, 200)
		require.Len(t, casbin.enforceCalls, 1)
		require.Equal(t, "user:1001", casbin.enforceCalls[0].sub)
	})

	t.Run("casbin error is propagated", func(t *testing.T) {
		casbin := &casbinFake{
			enforceFn: func(context.Context, string, string, string, string) (bool, error) {
				return false, perrors.WithCode(code.ErrInternalServerError, "enforce failed")
			},
		}
		handler := NewCheckHandler(casbin)

		recorder, _ := performAuthzRequest(http.MethodPost, "/check", `{"object":"scale:form:*","action":"read"}`, handler.Check, withTenantUser("tenant-a", "1001"))

		requireAuthzCode(t, recorder, http.StatusInternalServerError, code.ErrInternalServerError)
		require.Len(t, casbin.enforceCalls, 1)
	})
}
