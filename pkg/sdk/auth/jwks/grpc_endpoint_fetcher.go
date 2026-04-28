package jwks

import (
	"context"
	"fmt"
	"sync"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCEndpointFetcher 通过 endpoint 直接获取 JWKS。
type GRPCEndpointFetcher struct {
	endpoint string
	conn     *grpc.ClientConn
	client   authnv1.JWKSServiceClient
	next     KeyFetcher
	stats    *FetcherStats

	initOnce sync.Once
	initErr  error
}

// GRPCEndpointFetcherOption gRPC Endpoint Fetcher 配置选项。
type GRPCEndpointFetcherOption func(*GRPCEndpointFetcher)

// WithGRPCEndpointNext 设置下一个 fetcher。
func WithGRPCEndpointNext(next KeyFetcher) GRPCEndpointFetcherOption {
	return func(f *GRPCEndpointFetcher) {
		f.next = next
	}
}

// NewGRPCEndpointFetcher 创建通过 endpoint 连接的 gRPC Fetcher。
func NewGRPCEndpointFetcher(endpoint string, opts ...GRPCEndpointFetcherOption) *GRPCEndpointFetcher {
	f := &GRPCEndpointFetcher{
		endpoint: endpoint,
		stats:    &FetcherStats{},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *GRPCEndpointFetcher) Name() string {
	return "grpc-endpoint"
}

func (f *GRPCEndpointFetcher) init(ctx context.Context) error {
	f.initOnce.Do(func() {
		conn, err := grpc.DialContext(ctx, f.endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			f.initErr = fmt.Errorf("grpc-endpoint: dial %s: %w", f.endpoint, err)
			return
		}
		f.conn = conn
		f.client = authnv1.NewJWKSServiceClient(conn)
	})
	return f.initErr
}

func (f *GRPCEndpointFetcher) Fetch(ctx context.Context) (jwk.Set, error) {
	f.stats.IncrAttempts()

	if err := f.init(ctx); err != nil {
		return f.tryNext(ctx, err)
	}

	resp, err := f.client.GetJWKS(ctx, &authnv1.GetJWKSRequest{})
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

func (f *GRPCEndpointFetcher) tryNext(ctx context.Context, err error) (jwk.Set, error) {
	f.stats.IncrFailures()
	if f.next != nil {
		return f.next.Fetch(ctx)
	}
	return nil, fmt.Errorf("grpc-endpoint fetcher failed: %w", err)
}

// Close 关闭连接。
func (f *GRPCEndpointFetcher) Close() error {
	if f.conn != nil {
		return f.conn.Close()
	}
	return nil
}

// Stats 返回统计信息。
func (f *GRPCEndpointFetcher) Stats() FetcherStats {
	return f.stats.Snapshot()
}
