package sdk_test

import (
	"context"
	"testing"
	"time"

	sdk "github.com/FangcunMount/iam/pkg/sdk"
	authclient "github.com/FangcunMount/iam/pkg/sdk/auth/client"
	authjwks "github.com/FangcunMount/iam/pkg/sdk/auth/jwks"
	authserviceauth "github.com/FangcunMount/iam/pkg/sdk/auth/serviceauth"
	authverifier "github.com/FangcunMount/iam/pkg/sdk/auth/verifier"
	"github.com/FangcunMount/iam/pkg/sdk/authz"
	"github.com/FangcunMount/iam/pkg/sdk/config"
	sdkerrors "github.com/FangcunMount/iam/pkg/sdk/errors"
	"github.com/FangcunMount/iam/pkg/sdk/identity"
	"github.com/FangcunMount/iam/pkg/sdk/idp"
)

type compileMetrics struct{}

func (m *compileMetrics) RecordRequest(method, code string, duration time.Duration) {}

type compileTracing struct{}

func (t *compileTracing) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	return ctx, func() {}
}

func (t *compileTracing) SetAttributes(context.Context, map[string]string) {}
func (t *compileTracing) RecordError(context.Context, error)               {}

func TestPublicAPISurfaceCompiles(t *testing.T) {
	t.Parallel()

	var _ *sdk.Client
	var _ = sdk.NewClient
	var _ = sdk.WithRequestID
	var _ = sdk.WithTraceID
	var _ = sdk.GetRequestID
	var _ = sdk.GetTraceID
	var _ = sdk.ConfigFromEnv
	var _ = sdk.ConfigFromViper
	var _ = sdk.NewViperLoader
	var _ = sdk.DefaultObservabilityConfig

	var _ *sdk.Config
	var _ *sdk.TLSConfig
	var _ *sdk.RetryConfig
	var _ *sdk.JWKSConfig
	var _ *sdk.TokenVerifyConfig
	var _ *sdk.CircuitBreakerConfig
	var _ *sdk.ObservabilityConfig
	var _ *sdk.ServiceAuthConfig

	var _ sdk.MetricsCollector = (*compileMetrics)(nil)
	var _ sdk.TracingHook = (*compileTracing)(nil)
	var _ config.MetricsCollector = (*compileMetrics)(nil)
	var _ config.TracingHook = (*compileTracing)(nil)

	var opt sdk.ClientOption
	opt = sdk.WithMetricsCollector(&compileMetrics{})
	_ = opt
	opt = sdk.WithTracingHook(&compileTracing{})
	_ = opt

	var _ = authclient.NewClient
	var _ = authjwks.NewJWKSManager
	var _ = authverifier.NewTokenVerifier
	var _ = authserviceauth.NewServiceAuthHelper

	var _ *authz.Client
	var _ = authz.NewClient
	var _ *identity.Client
	var _ = identity.NewClient
	var _ *identity.GuardianshipClient
	var _ = identity.NewGuardianshipClient
	var _ *idp.Client
	var _ = idp.NewClient

	var _ = sdkerrors.Wrap
	var _ = sdkerrors.WrapWithCode
	var _ = sdkerrors.IsNotFound
	var _ = sdkerrors.IsRetryable
	var _ = sdkerrors.AsIAMError
	var _ = sdkerrors.GRPCCode
	var _ = sdkerrors.Message
	var _ = sdkerrors.ToHTTPStatus
}
