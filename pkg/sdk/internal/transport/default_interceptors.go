package transport

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	"github.com/FangcunMount/iam/v2/pkg/sdk/internal/observability"
	"google.golang.org/grpc"
)

func buildDefaultUnaryInterceptors(cfg *config.Config, opts *config.ClientOptions, result *DialResult) []grpc.UnaryClientInterceptor {
	if opts != nil && opts.DisableDefaultInterceptors {
		return nil
	}
	if cfg == nil || cfg.Observability == nil {
		return nil
	}

	obsCfg := cfg.Observability
	var interceptors []grpc.UnaryClientInterceptor

	if obsCfg.EnableRequestID {
		interceptors = append(interceptors, RequestIDInterceptor())
	}

	if obsCfg.EnableMetrics {
		collector := resolveMetricsCollector(obsCfg, opts, result)
		interceptors = append(interceptors, observability.MetricsUnaryInterceptor(collector))
	}

	if obsCfg.EnableCircuitBreaker {
		cb := resolveCircuitBreaker(cfg, result)
		interceptors = append(interceptors, observability.CircuitBreakerInterceptor(cb))
	}

	if obsCfg.EnableTracing && opts != nil && opts.TracingHook != nil {
		interceptors = append(interceptors, observability.TracingUnaryInterceptor(opts.TracingHook))
	}

	return interceptors
}

func mergeUnaryInterceptors(defaults []grpc.UnaryClientInterceptor, opts *config.ClientOptions) []grpc.UnaryClientInterceptor {
	all := append([]grpc.UnaryClientInterceptor{}, defaults...)
	if opts != nil && len(opts.UnaryInterceptors) > 0 {
		all = append(all, opts.UnaryInterceptors...)
	}
	return all
}

func withDialTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func resolveMetricsCollector(obsCfg *config.ObservabilityConfig, opts *config.ClientOptions, result *DialResult) config.MetricsCollector {
	if opts != nil && opts.MetricsCollector != nil {
		return opts.MetricsCollector
	}

	metrics := observability.NewPrometheusMetrics(
		observability.WithPrometheusNamespace(obsCfg.MetricsNamespace),
		observability.WithPrometheusSubsystem(obsCfg.MetricsSubsystem),
	)
	result.Metrics = metrics
	return metrics
}

func resolveCircuitBreaker(cfg *config.Config, result *DialResult) *observability.CircuitBreaker {
	if cfg != nil && cfg.CircuitBreaker != nil {
		cb := observability.NewCircuitBreaker(&observability.CircuitBreakerConfig{
			FailureThreshold: cfg.CircuitBreaker.FailureThreshold,
			OpenDuration:     cfg.CircuitBreaker.OpenDuration,
			HalfOpenRequests: cfg.CircuitBreaker.HalfOpenRequests,
			SuccessThreshold: cfg.CircuitBreaker.SuccessThreshold,
		})
		result.CircuitBreaker = cb
		return cb
	}

	cb := observability.NewCircuitBreaker(nil)
	result.CircuitBreaker = cb
	return cb
}
