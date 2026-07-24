package sdk

import (
	"context"

	internaltransport "github.com/FangcunMount/iam/v2/pkg/sdk/internal/transport"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return internaltransport.WithRequestID(ctx, requestID)
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return internaltransport.WithTraceID(ctx, traceID)
}

func GetRequestID(ctx context.Context) string {
	return internaltransport.GetRequestID(ctx)
}

func GetTraceID(ctx context.Context) string {
	return internaltransport.GetTraceID(ctx)
}
