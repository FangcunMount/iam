package authorization_test

import (
	"context"
	"testing"
	"time"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type routeDecisionRuntimeStub struct {
	request authorizationdomain.Request
}

func (s *routeDecisionRuntimeStub) Check(_ context.Context, request authorizationdomain.Request) (authorizationdomain.Decision, error) {
	s.request = request
	return authorizationdomain.Allow(meta.FromUint64(1), "reader", 7, time.Now()), nil
}

func TestRouteDecisionServiceBuildsCanonicalRequest(t *testing.T) {
	runtime := &routeDecisionRuntimeStub{}
	service := authorizationapp.NewRouteDecisionService(authorizationapp.NewDecisionService(runtime))

	allowed, err := service.CheckRoutePermission(
		context.Background(), "user:42", "tenant-a", "iam:authz:collection:roles", "read",
	)

	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, "user:42", runtime.request.Subject.String())
	require.Equal(t, "tenant-a", runtime.request.TenantIDString())
	require.Equal(t, "iam:authz:collection:roles", string(runtime.request.ResourceKey))
	require.Equal(t, "read", string(runtime.request.Action))
}
