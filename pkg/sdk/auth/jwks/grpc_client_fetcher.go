package jwks

import (
	"context"
	"fmt"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// GRPCFetcher 通过 gRPC client 获取 JWKS。
type GRPCFetcher struct {
	client JWKSClient
	next   KeyFetcher
	stats  *FetcherStats
}

// GRPCFetcherOption gRPC Fetcher 配置选项。
type GRPCFetcherOption func(*GRPCFetcher)

// WithGRPCNext 设置下一个 fetcher。
func WithGRPCNext(next KeyFetcher) GRPCFetcherOption {
	return func(f *GRPCFetcher) {
		f.next = next
	}
}

// NewGRPCFetcher 创建 gRPC Fetcher。
func NewGRPCFetcher(client JWKSClient, opts ...GRPCFetcherOption) *GRPCFetcher {
	f := &GRPCFetcher{
		client: client,
		stats:  &FetcherStats{},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *GRPCFetcher) Name() string {
	return "grpc"
}

func (f *GRPCFetcher) Fetch(ctx context.Context) (jwk.Set, error) {
	if f.client == nil {
		return f.tryNext(ctx, fmt.Errorf("grpc: client not configured"))
	}

	f.stats.IncrAttempts()
	resp, err := f.client.GetJWKS(ctx, &authnv2.GetJWKSRequest{})
	if err != nil {
		return f.tryNext(ctx, err)
	}

	keySet, err := parseJWKSResponse(resp.GetJwks())
	if err != nil {
		return f.tryNext(ctx, err)
	}

	f.stats.IncrSuccesses()
	return keySet, nil
}

func (f *GRPCFetcher) tryNext(ctx context.Context, err error) (jwk.Set, error) {
	f.stats.IncrFailures()
	if f.next != nil {
		return f.next.Fetch(ctx)
	}
	return nil, fmt.Errorf("grpc fetcher failed: %w", err)
}

// Stats 返回统计信息。
func (f *GRPCFetcher) Stats() FetcherStats {
	return f.stats.Snapshot()
}
