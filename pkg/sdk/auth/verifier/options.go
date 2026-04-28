package verifier

import "github.com/FangcunMount/iam/pkg/sdk/config"

// WithVerifyStrategy 设置验证策略。
func WithVerifyStrategy(strategy VerifyStrategy) TokenVerifierOption {
	return func(v *TokenVerifier) {
		v.strategy = strategy
	}
}

// WithVerifyConfig 设置验证配置。
func WithVerifyConfig(cfg *config.TokenVerifyConfig) TokenVerifierOption {
	return func(v *TokenVerifier) {
		v.config = cfg
	}
}
