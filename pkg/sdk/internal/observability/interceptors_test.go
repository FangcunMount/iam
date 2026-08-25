package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingMetricsCollector struct {
	method string
	code   string
	calls  int
}

func (c *recordingMetricsCollector) RecordRequest(method string, code string, duration time.Duration) {
	c.method = method
	c.code = code
	c.calls++
}

type recordingTracingHook struct {
	started   bool
	attrs     map[string]string
	recorded  error
	completed bool
}

func (h *recordingTracingHook) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	h.started = true
	return ctx, func() { h.completed = true }
}

func (h *recordingTracingHook) SetAttributes(ctx context.Context, attrs map[string]string) {
	h.attrs = attrs
}

func (h *recordingTracingHook) RecordError(ctx context.Context, err error) {
	h.recorded = err
}

func TestMetricsUnaryInterceptorRecordsMethodAndCode(t *testing.T) {
	t.Parallel()

	collector := &recordingMetricsCollector{}
	interceptor := MetricsUnaryInterceptor(collector)
	err := interceptor(context.Background(), "/iam.authz.v3.AuthorizationService/Check", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Error(codes.PermissionDenied, "denied")
	})
	require.Error(t, err)
	require.Equal(t, 1, collector.calls)
	require.Equal(t, "/iam.authz.v3.AuthorizationService/Check", collector.method)
	require.Equal(t, codes.PermissionDenied.String(), collector.code)
}

func TestTracingUnaryInterceptorDelegatesToHook(t *testing.T) {
	t.Parallel()

	hook := &recordingTracingHook{}
	interceptor := TracingUnaryInterceptor(hook)
	err := interceptor(context.Background(), "/iam.identity.v2.IdentityRead/GetUser", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Error(codes.Internal, "boom")
	})
	require.Error(t, err)
	require.True(t, hook.started)
	require.True(t, hook.completed)
	require.Equal(t, "grpc", hook.attrs["rpc.system"])
	require.Equal(t, "iam.identity.v2.IdentityRead", hook.attrs["rpc.service"])
	require.Equal(t, "GetUser", hook.attrs["rpc.method"])
	require.Equal(t, "rpc error: code = Internal desc = boom", hook.recorded.Error())
}

func TestCircuitBreakerInterceptorOpensAfterFailureThreshold(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenDuration:     time.Minute,
		HalfOpenRequests: 1,
		SuccessThreshold: 1,
		FailureCodes:     []codes.Code{codes.Unavailable},
	})
	interceptor := CircuitBreakerInterceptor(cb)

	err := interceptor(context.Background(), "/iam.authn.v2.AuthService/VerifyToken", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Error(codes.Unavailable, "boom")
	})
	require.Error(t, err)
	require.Equal(t, CircuitOpen, cb.State())

	err = interceptor(context.Background(), "/iam.authn.v2.AuthService/VerifyToken", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		t.Fatal("invoker should not run when circuit is open")
		return nil
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
