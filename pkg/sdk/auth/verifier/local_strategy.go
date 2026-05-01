package verifier

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/logger"
	authjwks "github.com/FangcunMount/iam/v2/pkg/sdk/auth/jwks"
	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	iamerrors "github.com/FangcunMount/iam/v2/pkg/sdk/errors"
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
		return nil, fmt.Errorf("local-strategy: get keys: %w", err)
	}

	var verifyOpts []jwt.ParseOption
	algorithms := s.getAllowedAlgorithms()
	if len(algorithms) == 1 {
		verifyOpts = append(verifyOpts, jwt.WithKeySet(keySet, algorithms[0]))
	} else {
		verifyOpts = append(verifyOpts, jwt.WithKeySet(keySet))
	}

	audience := opts.ExpectedAudience
	if len(audience) == 0 && s.config != nil {
		audience = s.config.AllowedAudience
	}
	if len(audience) > 0 {
		for _, aud := range audience {
			verifyOpts = append(verifyOpts, jwt.WithAudience(aud))
		}
	}

	issuer := opts.ExpectedIssuer
	if issuer == "" && s.config != nil {
		issuer = s.config.AllowedIssuer
	}
	if issuer != "" {
		verifyOpts = append(verifyOpts, jwt.WithIssuer(issuer))
	}

	if s.config != nil && s.config.ClockSkew > 0 {
		verifyOpts = append(verifyOpts, jwt.WithAcceptableSkew(s.config.ClockSkew))
	}
	if s.config != nil && len(s.config.RequiredClaims) > 0 {
		for _, claim := range s.config.RequiredClaims {
			verifyOpts = append(verifyOpts, jwt.WithRequiredClaim(claim))
		}
	}

	token, err := jwt.Parse([]byte(tokenString), verifyOpts...)
	if err != nil {
		if jwt.IsValidationError(err) {
			return nil, iamerrors.ErrTokenExpired
		}
		return nil, fmt.Errorf("local-strategy: parse token: %w", err)
	}

	claims := extractClaims(token)
	return &VerifyResult{
		Valid:    true,
		Claims:   claims,
		Metadata: buildVerifyMetadataFromClaims(claims),
		RawToken: token,
	}, nil
}
