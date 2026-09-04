package verifier

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/logger"
)

// CachingVerifyStrategy 缓存验证结果的策略。
type CachingVerifyStrategy struct {
	delegate VerifyStrategy
	cache    VerifyResultCache
	ttl      time.Duration
}

// NewCachingVerifyStrategy 创建缓存策略。
func NewCachingVerifyStrategy(delegate VerifyStrategy, cache VerifyResultCache, ttl time.Duration) *CachingVerifyStrategy {
	return &CachingVerifyStrategy{
		delegate: delegate,
		cache:    cache,
		ttl:      ttl,
	}
}

func (s *CachingVerifyStrategy) Name() string {
	return fmt.Sprintf("caching(%s)", s.delegate.Name())
}

func (s *CachingVerifyStrategy) Verify(ctx context.Context, token string, opts *VerifyOptions) (*VerifyResult, error) {
	logger.L(ctx).Debugw("CachingVerifyStrategy verify start", "delegate", s.delegate.Name(), "ttl", s.ttl.String())
	if cached, ok := s.cache.Get(token); ok {
		logger.L(ctx).Debugw("CachingVerifyStrategy cache hit", "delegate", s.delegate.Name())
		if cached == nil || cached.Claims == nil {
			return nil, invalidTokenError("cached verification result has no claims")
		}
		if err := newVerificationPolicy(nil, opts).validateTokenType(cached.Claims.TokenType); err != nil {
			return nil, err
		}
		return cached, nil
	}

	result, err := s.delegate.Verify(ctx, token, opts)
	if err != nil {
		logger.L(ctx).Errorw("CachingVerifyStrategy delegate verify failed", "delegate", s.delegate.Name(), "error", err.Error())
		return nil, fmt.Errorf("caching-strategy: delegate verify failed: %w", err)
	}

	s.cache.Set(token, result, s.ttl)
	return result, nil
}
