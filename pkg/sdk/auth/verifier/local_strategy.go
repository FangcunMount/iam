package verifier

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/logger"
	authjwks "github.com/FangcunMount/iam/v3/pkg/sdk/auth/jwks"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// LocalVerifyStrategy 本地验证策略（使用 JWKS）。
type LocalVerifyStrategy struct {
	config      *config.TokenVerifyConfig
	jwksManager *authjwks.JWKSManager
}

// LocalStrategyOption 本地策略配置选项。
type LocalStrategyOption func(*LocalVerifyStrategy)

// WithLocalConfig 设置验证配置。
func WithLocalConfig(cfg *config.TokenVerifyConfig) LocalStrategyOption {
	return func(s *LocalVerifyStrategy) {
		s.config = cfg
	}
}

// NewLocalVerifyStrategy 创建本地验证策略。
func NewLocalVerifyStrategy(jwksManager *authjwks.JWKSManager, opts ...LocalStrategyOption) *LocalVerifyStrategy {
	s := &LocalVerifyStrategy{
		config:      &config.TokenVerifyConfig{},
		jwksManager: jwksManager,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *LocalVerifyStrategy) Name() string {
	return "local"
}

func (s *LocalVerifyStrategy) Verify(ctx context.Context, tokenString string, opts *VerifyOptions) (*VerifyResult, error) {
	if opts == nil {
		opts = &VerifyOptions{}
	}
	logger.L(ctx).Debugw("LocalVerifyStrategy verify start", "strategy", s.Name(), "has_jwks_manager", s.jwksManager != nil)
	if s.jwksManager == nil {
		logger.L(ctx).Errorw("LocalVerifyStrategy jwks manager not configured", "strategy", s.Name())
		return nil, fmt.Errorf("local-strategy: jwks manager not configured")
	}

	keySet, err := s.jwksManager.GetKeySet(ctx)
	if err != nil {
		logger.L(ctx).Errorw("LocalVerifyStrategy get keys failed", "strategy", s.Name(), "error", err.Error())
		return nil, allowRemoteFallback(fmt.Errorf("local-strategy: get keys: %w", err))
	}
	if keySet == nil || keySet.Len() == 0 {
		return nil, allowRemoteFallback(fmt.Errorf("local-strategy: jwks key set is empty"))
	}

	policy := newVerificationPolicy(s.config, opts)
	if err := policy.validateAlgorithm(tokenString); err != nil {
		return nil, err
	}
	verifyOpts := policy.appendParseOptions([]jwt.ParseOption{jwt.WithKeySet(keySet)})

	token, err := jwt.Parse([]byte(tokenString), verifyOpts...)
	if err != nil {
		return nil, mapTokenValidationError(err)
	}

	claims := extractClaims(token)
	return &VerifyResult{
		Valid:    true,
		Claims:   claims,
		Metadata: buildVerifyMetadataFromClaims(claims),
		RawToken: token,
	}, nil
}
