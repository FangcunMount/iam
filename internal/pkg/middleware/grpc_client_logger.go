package middleware

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/FangcunMount/component-base/pkg/log"
)

// UnaryClientLoggingInterceptor gRPC一元客户端日志拦截器
func UnaryClientLoggingInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		startTime := time.Now()

		// 记录请求开始
		grpcInfo(ctx, "gRPC client request started",
			log.String("method", method),
			log.String("target", cc.Target()),
		)

		// 执行gRPC调用
		err := invoker(ctx, method, req, reply, cc, opts...)

		// 计算耗时
		latency := time.Since(startTime)

		// 记录响应信息
		if err != nil {
			st := status.Convert(err)
			grpcError(ctx, "gRPC client request failed",
				log.String("method", method),
				log.String("code", st.Code().String()),
				log.Duration("latency", latency),
			)
		} else {
			grpcInfo(ctx, "gRPC client request succeeded",
				log.String("method", method),
				log.Duration("latency", latency),
			)

		}

		grpcInfo(ctx, "gRPC client request completed",
			log.String("method", method),
			log.Duration("latency", latency),
		)
		return err
	}
}

// StreamClientLoggingInterceptor gRPC流式客户端日志拦截器
func StreamClientLoggingInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		grpcInfo(ctx, "gRPC stream started",
			log.String("method", method),
			log.String("target", cc.Target()),
			log.Bool("server_streams", desc.ServerStreams),
			log.Bool("client_streams", desc.ClientStreams),
		)

		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			st := status.Convert(err)
			grpcError(ctx, "gRPC stream error",
				log.String("method", method),
				log.String("code", st.Code().String()),
			)
			return nil, err
		}

		grpcInfo(ctx, "gRPC stream established successfully", log.String("method", method))
		return &loggingClientStream{ClientStream: stream, ctx: ctx, method: method}, nil
	}
}

// loggingClientStream 包装gRPC流以添加日志功能
type loggingClientStream struct {
	grpc.ClientStream
	ctx    context.Context
	method string
}

func (s *loggingClientStream) SendMsg(m interface{}) error {
	grpcDebug(s.ctx, "gRPC stream send message", log.String("method", s.method))
	return s.ClientStream.SendMsg(m)
}

func (s *loggingClientStream) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		grpcWarn(s.ctx, "gRPC stream receive error",
			log.String("method", s.method),
		)
	} else {
		grpcDebug(s.ctx, "gRPC stream receive success", log.String("method", s.method))
	}
	return err
}

func (s *loggingClientStream) CloseSend() error {
	grpcDebug(s.ctx, "gRPC stream close send", log.String("method", s.method))
	return s.ClientStream.CloseSend()
}

func grpcInfo(ctx context.Context, msg string, fields ...log.Field) {
	log.GRPC(msg, append(fields, log.TraceFields(ctx)...)...)
}

func grpcDebug(ctx context.Context, msg string, fields ...log.Field) {
	log.GRPCDebug(msg, append(fields, log.TraceFields(ctx)...)...)
}

func grpcWarn(ctx context.Context, msg string, fields ...log.Field) {
	log.GRPCWarn(msg, append(fields, log.TraceFields(ctx)...)...)
}

func grpcError(ctx context.Context, msg string, fields ...log.Field) {
	log.GRPCError(msg, append(fields, log.TraceFields(ctx)...)...)
}
