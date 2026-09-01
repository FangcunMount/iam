package verifier

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/logger"
)

// FallbackVerifyStrategy 降级策略。
type FallbackVerifyStrategy struct {
	primary  VerifyStrategy
	fallback VerifyStrategy
}

// NewFallbackVerifyStrategy 创建降级策略。
func NewFallbackVerifyStrategy(primary, fallback VerifyStrategy) *FallbackVerifyStrategy {
	return &FallbackVerifyStrategy{
		primary:  primary,
		fallback: fallback,
	}
}

func (s *FallbackVerifyStrategy) Name() string {
	return fmt.Sprintf("fallback(%s->%s)", s.primary.Name(), s.fallback.Name())
}

func (s *FallbackVerifyStrategy) Verify(ctx context.Context, token string, opts *VerifyOptions) (*VerifyResult, error) {
	logger.L(ctx).Debugw("FallbackVerifyStrategy verify start", "primary", s.primary.Name(), "has_fallback", s.fallback != nil)
	result, err := s.primary.Verify(ctx, token, opts)
	if err == nil {
		logger.L(ctx).Debugw("FallbackVerifyStrategy primary verify success", "primary", s.primary.Name())
		return result, nil
	}
	if !canFallbackRemotely(err) {
		logger.L(ctx).Debugw("FallbackVerifyStrategy primary rejected token without fallback", "primary", s.primary.Name(), "error", err.Error())
		return nil, err
	}

	if s.fallback != nil {
		logger.L(ctx).Warnw("FallbackVerifyStrategy primary verify failed, trying fallback", "primary", s.primary.Name(), "fallback", s.fallback.Name(), "error", err.Error())
		result, err := s.fallback.Verify(ctx, token, opts)
		if err == nil {
			logger.L(ctx).Warnw("FallbackVerifyStrategy fallback verify success", "fallback", s.fallback.Name())
			return result, nil
		}
		logger.L(ctx).Errorw("FallbackVerifyStrategy fallback verify failed", "fallback", s.fallback.Name(), "error", err.Error())
		return nil, err
	}

	logger.L(ctx).Errorw("FallbackVerifyStrategy verify failed without fallback", "primary", s.primary.Name(), "error", err.Error())
	return nil, err
}
