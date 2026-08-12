package jwks

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
)

// JWKSManagerOption JWKSManager 配置选项。
type JWKSManagerOption func(*managerBuilder)

type managerBuilder struct {
	config               *config.JWKSConfig
	authClient           JWKSClient
	cbConfig             *config.CircuitBreakerConfig
	seedData             []byte
	customChain          KeyFetcher
	enableGRPC           bool
	enableCache          bool
	enableCircuitBreaker bool
}

// WithAuthClient 设置 gRPC 客户端用于降级。
func WithAuthClient(client JWKSClient) JWKSManagerOption {
	return func(b *managerBuilder) {
		b.authClient = client
		b.enableGRPC = true
	}
}

// WithCircuitBreakerConfig 启用熔断器。
func WithCircuitBreakerConfig(cfg *config.CircuitBreakerConfig) JWKSManagerOption {
	return func(b *managerBuilder) {
		b.cbConfig = cfg
		b.enableCircuitBreaker = true
	}
}

// WithSeedData 设置种子数据。
func WithSeedData(data []byte) JWKSManagerOption {
	return func(b *managerBuilder) {
		b.seedData = data
	}
}

// WithCustomChain 设置自定义职责链（覆盖默认链）。
func WithCustomChain(chain KeyFetcher) JWKSManagerOption {
	return func(b *managerBuilder) {
		b.customChain = chain
	}
}

// WithCacheEnabled 启用缓存。
func WithCacheEnabled(enabled bool) JWKSManagerOption {
	return func(b *managerBuilder) {
		b.enableCache = enabled
	}
}

// NewJWKSManager 创建 JWKS 管理器。
//
// 默认职责链: Cache -> CircuitBreaker -> HTTP -> gRPC -> Seed
func NewJWKSManager(cfg *config.JWKSConfig, opts ...JWKSManagerOption) (*JWKSManager, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, fmt.Errorf("jwks: url is required")
	}

	builder := newManagerBuilder(cfg)
	for _, opt := range opts {
		opt(builder)
	}

	chain := builder.buildChain()
	cache := builder.resolveCache(chain)

	m := &JWKSManager{
		config: cfg,
		chain:  chain,
		cache:  cache,
		stopCh: make(chan struct{}),
	}

	if _, err := m.chain.Fetch(context.Background()); err != nil {
		return nil, fmt.Errorf("jwks: initial fetch failed: %w", err)
	}

	if m.shouldStartRefreshLoop() {
		go m.refreshLoop()
	}

	return m, nil
}

func newManagerBuilder(cfg *config.JWKSConfig) *managerBuilder {
	return &managerBuilder{
		config:      cfg,
		enableCache: true,
	}
}

func (b *managerBuilder) buildChain() KeyFetcher {
	if b.customChain != nil {
		return b.customChain
	}
	return buildDefaultChain(b)
}

func (b *managerBuilder) resolveCache(chain KeyFetcher) *CacheFetcher {
	if !b.enableCache {
		return nil
	}
	if cf, ok := chain.(*CacheFetcher); ok {
		return cf
	}
	return nil
}
