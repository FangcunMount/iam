package transport

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type testMetricsCollector struct {
	calls []string
}

func (c *testMetricsCollector) RecordRequest(method string, code string, duration time.Duration) {
	c.calls = append(c.calls, method+":"+code)
}

type testTracingHook struct {
	started bool
}

func (h *testTracingHook) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	h.started = true
	return ctx, func() {}
}

func (h *testTracingHook) SetAttributes(context.Context, map[string]string) {}
func (h *testTracingHook) RecordError(context.Context, error)               {}

func TestBuildDefaultUnaryInterceptorsRequiresExplicitObservability(t *testing.T) {
	t.Parallel()

	interceptors := buildDefaultUnaryInterceptors(&config.Config{}, &config.ClientOptions{
		MetricsCollector: &testMetricsCollector{},
		TracingHook:      &testTracingHook{},
	}, &DialResult{})
	require.Empty(t, interceptors)
}

func TestBuildDefaultUnaryInterceptorsRespectsExplicitObservability(t *testing.T) {
	t.Parallel()

	collector := &testMetricsCollector{}
	tracing := &testTracingHook{}
	result := &DialResult{}
	cfg := &config.Config{
		Observability: &config.ObservabilityConfig{
			EnableRequestID:      true,
			EnableMetrics:        true,
			EnableCircuitBreaker: true,
			EnableTracing:        true,
			MetricsNamespace:     "iam",
			MetricsSubsystem:     "sdk",
		},
	}
	opts := &config.ClientOptions{
		MetricsCollector: collector,
		TracingHook:      tracing,
	}

	interceptors := buildDefaultUnaryInterceptors(cfg, opts, result)
	require.Len(t, interceptors, 4)
	require.NotNil(t, result.CircuitBreaker)
	require.Nil(t, result.Metrics)

	md := invokeUnaryChain(t, interceptors, context.Background(), nil)
	require.Len(t, md.Get(MetadataKeyRequestID), 1)
	require.True(t, tracing.started)
	require.NotEmpty(t, collector.calls)
}

func TestWithRequestIDAndTraceIDWriteOutgoingMetadata(t *testing.T) {
	t.Parallel()

	ctx := WithRequestID(context.Background(), "req-123")
	ctx = WithTraceID(ctx, "trace-abc")

	require.Equal(t, "req-123", GetRequestID(ctx))
	require.Equal(t, "trace-abc", GetTraceID(ctx))

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"req-123"}, md.Get(MetadataKeyRequestID))
	require.Equal(t, []string{"trace-abc"}, md.Get(MetadataKeyTraceID))
}

func invokeUnaryChain(t *testing.T, interceptors []grpc.UnaryClientInterceptor, ctx context.Context, err error) metadata.MD {
	t.Helper()

	var captured metadata.MD
	var call func(int, context.Context) error
	call = func(idx int, current context.Context) error {
		if idx == len(interceptors) {
			md, ok := metadata.FromOutgoingContext(current)
			require.True(t, ok)
			captured = md
			return err
		}
		return interceptors[idx](current, "/iam.authn.v2.AuthService/VerifyToken", nil, nil, nil, func(nextCtx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			return call(idx+1, nextCtx)
		})
	}

	require.NoError(t, call(0, ctx))
	return captured
}
