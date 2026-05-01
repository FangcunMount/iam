package transport

import (
	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// BuildDialOptions 构建 gRPC DialOption（不包含拦截器，拦截器由 Dial 统一处理）
func BuildDialOptions(cfg *config.Config, opts *config.ClientOptions) ([]grpc.DialOption, error) {
	var dialOpts []grpc.DialOption

	tlsCreds, err := BuildTLSCredentials(cfg.TLS)
	if err != nil {
		return nil, err
	}
	dialOpts = append(dialOpts, tlsCreds)

	if cfg.Keepalive != nil {
		dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                cfg.Keepalive.Time,
			Timeout:             cfg.Keepalive.Timeout,
			PermitWithoutStream: cfg.Keepalive.PermitWithoutStream,
		}))
	}

	serviceConfig := BuildServiceConfig(cfg)
	if serviceConfig != "" {
		dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(serviceConfig))
	}

	if opts != nil {
		if len(opts.StreamInterceptors) > 0 {
			dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(opts.StreamInterceptors...))
		}
		if len(opts.DialOptions) > 0 {
			dialOpts = append(dialOpts, opts.DialOptions...)
		}
	}

	return dialOpts, nil
}
