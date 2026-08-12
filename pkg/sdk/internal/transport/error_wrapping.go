package transport

import (
	"context"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"google.golang.org/grpc"
)

// ErrorWrappingInterceptor 错误包装拦截器，将 gRPC 错误转换为 IAMError。
func ErrorWrappingInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			return sdkerrors.Wrap(err)
		}
		return nil
	}
}

// ErrorWrappingStreamInterceptor 流式调用的错误包装拦截器。
func ErrorWrappingStreamInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, sdkerrors.Wrap(err)
		}
		return &wrappedClientStream{ClientStream: stream}, nil
	}
}

type wrappedClientStream struct {
	grpc.ClientStream
}

func (w *wrappedClientStream) SendMsg(m interface{}) error {
	err := w.ClientStream.SendMsg(m)
	if err != nil {
		return sdkerrors.Wrap(err)
	}
	return nil
}

func (w *wrappedClientStream) RecvMsg(m interface{}) error {
	err := w.ClientStream.RecvMsg(m)
	if err != nil {
		return sdkerrors.Wrap(err)
	}
	return nil
}
