// Package transport 提供 gRPC 传输层功能
package transport

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	"github.com/FangcunMount/iam/v2/pkg/sdk/internal/observability"
	"google.golang.org/grpc"
)

// DialResult Dial 的返回结果，包含连接和可观测性组件
type DialResult struct {
	Conn           *grpc.ClientConn
	Metrics        *observability.PrometheusMetrics
	CircuitBreaker *observability.CircuitBreaker
}

// Dial 创建 gRPC 连接
func Dial(ctx context.Context, cfg *config.Config, opts *config.ClientOptions) (*grpc.ClientConn, error) {
	result, err := DialWithObservability(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}
	return result.Conn, nil
}

// DialWithObservability 创建 gRPC 连接，并返回可观测性组件
func DialWithObservability(ctx context.Context, cfg *config.Config, opts *config.ClientOptions) (*DialResult, error) {
	result := &DialResult{}
	allInterceptors := mergeUnaryInterceptors(buildDefaultUnaryInterceptors(cfg, opts, result), opts)

	// 构建 DialOptions
	dialOpts, err := BuildDialOptions(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("build dial options: %w", err)
	}

	// 添加合并后的拦截器链
	if len(allInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(allInterceptors...))
	}

	dialCtx, cancel := withDialTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, cfg.Endpoint, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", cfg.Endpoint, err)
	}

	result.Conn = conn
	return result, nil
}
