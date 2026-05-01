package transport

import (
	"context"

	sdkerrors "github.com/FangcunMount/iam/v2/pkg/sdk/errors"
	internalerrors "github.com/FangcunMount/iam/v2/pkg/sdk/internal/errorsx"
	"google.golang.org/grpc"
)

// ErrorHandlerConfig 错误处理拦截器配置。
type ErrorHandlerConfig struct {
	OnError        func(ctx context.Context, method string, err error)
	TransformError func(err error) error
	IgnoreErrors   internalerrors.ErrorMatcher
}

// ErrorHandlerInterceptor 错误处理拦截器。
func ErrorHandlerInterceptor(cfg *ErrorHandlerConfig) grpc.UnaryClientInterceptor {
	if cfg == nil {
		cfg = &ErrorHandlerConfig{}
	}

	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		}

		if cfg.IgnoreErrors != nil && cfg.IgnoreErrors.Match(err) {
			return nil
		}
		if cfg.OnError != nil {
			cfg.OnError(ctx, method, err)
		}
		if cfg.TransformError != nil {
			return cfg.TransformError(err)
		}
		return sdkerrors.Wrap(err)
	}
}

// DefaultErrorHandlerConfig 默认错误处理配置。
func DefaultErrorHandlerConfig() *ErrorHandlerConfig {
	return &ErrorHandlerConfig{
		TransformError: sdkerrors.Wrap,
	}
}

// LoggingErrorHandlerConfig 带日志的错误处理配置。
func LoggingErrorHandlerConfig(logger func(method string, err error)) *ErrorHandlerConfig {
	return &ErrorHandlerConfig{
		OnError: func(ctx context.Context, method string, err error) {
			if logger != nil {
				logger(method, err)
			}
		},
		TransformError: sdkerrors.Wrap,
	}
}

// IgnoreNotFoundConfig 忽略 NotFound 错误的配置。
func IgnoreNotFoundConfig() *ErrorHandlerConfig {
	return &ErrorHandlerConfig{
		IgnoreErrors:   internalerrors.ResourceErrors,
		TransformError: sdkerrors.Wrap,
	}
}
