package jwks

import (
	"context"
	"sync"
	"time"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// JWKSClient 定义 gRPC JWKS 降级客户端的最小能力。
type JWKSClient interface {
	GetJWKS(context.Context, *authnv1.GetJWKSRequest) (*authnv1.GetJWKSResponse, error)
}

// KeyFetcher 定义获取 JWKS 的职责链节点。
type KeyFetcher interface {
	Fetch(ctx context.Context) (jwk.Set, error)
	Name() string
}

// FetcherStats fetcher 统计信息。
type FetcherStats struct {
	mu        sync.Mutex
	attempts  int64
	successes int64
	failures  int64
}

func (s *FetcherStats) IncrAttempts() {
	s.mu.Lock()
	s.attempts++
	s.mu.Unlock()
}

func (s *FetcherStats) IncrSuccesses() {
	s.mu.Lock()
	s.successes++
	s.mu.Unlock()
}

func (s *FetcherStats) IncrFailures() {
	s.mu.Lock()
	s.failures++
	s.mu.Unlock()
}

func (s *FetcherStats) Snapshot() FetcherStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return FetcherStats{
		attempts:  s.attempts,
		successes: s.successes,
		failures:  s.failures,
	}
}

// Attempts 返回尝试次数。
func (s *FetcherStats) Attempts() int64 { return s.attempts }

// Successes 返回成功次数。
func (s *FetcherStats) Successes() int64 { return s.successes }

// Failures 返回失败次数。
func (s *FetcherStats) Failures() int64 { return s.failures }

// JWKSStats 兼容旧接口的聚合统计结构。
type JWKSStats struct {
	HTTPFetches   int64
	GRPCFetches   int64
	CacheHits     int64
	SeedCacheHits int64
	LastUpdate    time.Time
	State         CircuitState
}
