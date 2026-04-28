package verifier

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	authjwks "github.com/FangcunMount/iam/pkg/sdk/auth/jwks"
	"github.com/FangcunMount/iam/pkg/sdk/config"
)

// NewTokenVerifier 创建 Token 验证器。
func NewTokenVerifier(cfg *config.TokenVerifyConfig, jwksManager *authjwks.JWKSManager, authClient VerifyTokenClient) (*TokenVerifier, error) {
	logger.L(context.Background()).Debugw("NewTokenVerifier cfg", "cfg", cfg)
	logger.L(context.Background()).Debugw("NewTokenVerifier jwksManager", "jwksManager", jwksManager)
	logger.L(context.Background()).Debugw("NewTokenVerifier authClient", "authClient", authClient)

	selector := NewStrategySelector(cfg, jwksManager, authClient)
	strategy, err := selector.Select()
	logger.L(context.Background()).Debugw("NewTokenVerifier strategy", "strategy", strategy)
	if err != nil {
		return nil, err
	}

	var remoteStrategy VerifyStrategy
	if authClient != nil {
		remoteStrategy, err = selector.RemoteStrategy()
		if err != nil {
			return nil, err
		}
	}

	return &TokenVerifier{
		config:         cfg,
		strategy:       strategy,
		remoteStrategy: remoteStrategy,
	}, nil
}

// NewTokenVerifierWithStrategy 使用自定义策略创建验证器。
func NewTokenVerifierWithStrategy(strategy VerifyStrategy, opts ...TokenVerifierOption) *TokenVerifier {
	v := &TokenVerifier{
		config:   &config.TokenVerifyConfig{},
		strategy: strategy,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}
