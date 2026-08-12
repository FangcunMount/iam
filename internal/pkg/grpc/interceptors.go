// Package grpc 提供 IAM 特定的 gRPC 拦截器配置
//
// 本包基于 pkg/grpc/interceptors 提供的通用拦截器，
// 集成 component-base 的日志和追踪功能。
package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
)

// RecoveryInterceptor 返回集成 component-base 日志的恢复拦截器
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return interceptors.RecoveryInterceptor(
		interceptors.WithRecoveryLogger(&componentBaseLogger{}),
		interceptors.WithRecoveryContextPreprocess(ensureTraceContext),
	)
}

// RequestIDInterceptor 返回集成 component-base 的请求ID拦截器
func RequestIDInterceptor() grpc.UnaryServerInterceptor {
	return interceptors.RequestIDInterceptor(
		interceptors.WithRequestIDGenerator(idutil.NewRequestID),
		interceptors.WithMetadataInjector(func(ctx context.Context, requestID string) context.Context {
			return log.WithRequestID(ctx, requestID)
		}),
	)
}

// componentBaseLogger 适配 component-base 日志到 InterceptorLogger 接口
type componentBaseLogger struct{}

func (l *componentBaseLogger) LogInfo(msg string, fields map[string]interface{}) {
	logFields := mapToLogFields(fields)
	log.GRPC(msg, logFields...)
}

func (l *componentBaseLogger) LogError(msg string, fields map[string]interface{}) {
	logFields := mapToLogFields(fields)
	log.GRPCError(msg, logFields...)
}

func mapToLogFields(fields map[string]interface{}) []log.Field {
	logFields := make([]log.Field, 0, len(fields))
	for k, v := range fields {
		logFields = append(logFields, log.Any(k, v))
	}
	return logFields
}

// ensureTraceContext 确保上下文中包含追踪信息
func ensureTraceContext(ctx context.Context) context.Context {
	traceID := getMetadataValue(ctx, "x-trace-id")
	if traceID == "" {
		traceID = idutil.NewTraceID()
	}

	requestID := getMetadataValue(ctx, "x-request-id")
	if requestID == "" {
		requestID = idutil.NewRequestID()
	}

	spanID := idutil.NewSpanID()

	return log.WithTraceContext(ctx, traceID, spanID, requestID)
}

func getMetadataValue(ctx context.Context, key string) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get(key)
		if len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
