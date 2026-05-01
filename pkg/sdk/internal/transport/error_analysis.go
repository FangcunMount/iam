package transport

import (
	"context"

	internalerrors "github.com/FangcunMount/iam/v2/pkg/sdk/internal/errorsx"
	"google.golang.org/grpc"
)

// ErrorAnalyzer 错误分析回调。
type ErrorAnalyzer func(ctx context.Context, method string, details *internalerrors.ErrorDetails)

// ErrorAnalysisInterceptor 错误分析拦截器（用于观测）。
func ErrorAnalysisInterceptor(analyzer ErrorAnalyzer) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil && analyzer != nil {
			details := internalerrors.Analyze(err)
			analyzer(ctx, method, details)
		}
		return err
	}
}
