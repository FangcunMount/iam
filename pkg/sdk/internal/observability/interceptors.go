package observability

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// MetricsUnaryInterceptor 返回 Metrics 收集拦截器。
func MetricsUnaryInterceptor(collector config.MetricsCollector) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		code := "OK"
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code().String()
			} else {
				code = "UNKNOWN"
			}
		}

		collector.RecordRequest(method, code, duration)
		return err
	}
}

// TracingUnaryInterceptor 返回 Tracing 拦截器。
func TracingUnaryInterceptor(hook config.TracingHook) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, endSpan := hook.StartSpan(ctx, method)
		defer endSpan()

		hook.SetAttributes(ctx, map[string]string{
			"rpc.system":  "grpc",
			"rpc.service": extractService(method),
			"rpc.method":  extractMethod(method),
		})

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			hook.RecordError(ctx, err)
		}
		return err
	}
}
