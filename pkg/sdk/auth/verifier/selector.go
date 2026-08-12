package verifier

import (
	"fmt"

	authjwks "github.com/FangcunMount/iam/v3/pkg/sdk/auth/jwks"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
)

// StrategySelector 策略选择器，根据条件选择合适的验证策略。
type StrategySelector struct {
	cfg         *config.TokenVerifyConfig
	jwksManager *authjwks.JWKSManager
	authClient  VerifyTokenClient
}

// NewStrategySelector 创建策略选择器。
func NewStrategySelector(cfg *config.TokenVerifyConfig, jwksManager *authjwks.JWKSManager, authClient VerifyTokenClient) *StrategySelector {
	if cfg == nil {
		cfg = &config.TokenVerifyConfig{}
	}
	return &StrategySelector{
		cfg:         cfg,
		jwksManager: jwksManager,
		authClient:  authClient,
	}
}

// Select 根据配置选择最佳策略。
func (s *StrategySelector) Select() (VerifyStrategy, error) {
	hasJWKS := s.jwksManager != nil
	hasRemote := s.authClient != nil

	if !hasJWKS && !hasRemote {
		return nil, fmt.Errorf("strategy-selector: at least one of jwksManager or authClient is required")
	}

	switch {
	case s.cfg.ForceRemoteVerification:
		return s.selectRemoteOnly()
	case hasJWKS && hasRemote:
		return s.selectWithFallback()
	case hasJWKS:
		return s.selectLocalOnly()
	case hasRemote:
		return s.selectRemoteOnly()
	default:
		return nil, fmt.Errorf("strategy-selector: no valid strategy available")
	}
}

func (s *StrategySelector) selectLocalOnly() (VerifyStrategy, error) {
	if s.jwksManager == nil {
		return nil, fmt.Errorf("strategy-selector: jwks manager required for local strategy")
	}
	return NewLocalVerifyStrategy(s.jwksManager, WithLocalConfig(s.cfg)), nil
}

func (s *StrategySelector) selectRemoteOnly() (VerifyStrategy, error) {
	if s.authClient == nil {
		return nil, fmt.Errorf("strategy-selector: auth client required for remote strategy")
	}
	return NewRemoteVerifyStrategy(s.authClient, s.cfg), nil
}

func (s *StrategySelector) selectWithFallback() (VerifyStrategy, error) {
	localStrategy := NewLocalVerifyStrategy(s.jwksManager, WithLocalConfig(s.cfg))
	remoteStrategy := NewRemoteVerifyStrategy(s.authClient, s.cfg)
	return NewFallbackVerifyStrategy(localStrategy, remoteStrategy), nil
}

// LocalStrategy 显式获取本地策略。
func (s *StrategySelector) LocalStrategy() (*LocalVerifyStrategy, error) {
	if s.jwksManager == nil {
		return nil, fmt.Errorf("strategy-selector: jwks manager not available")
	}
	return NewLocalVerifyStrategy(s.jwksManager, WithLocalConfig(s.cfg)), nil
}

// RemoteStrategy 显式获取远程策略。
func (s *StrategySelector) RemoteStrategy() (*RemoteVerifyStrategy, error) {
	if s.authClient == nil {
		return nil, fmt.Errorf("strategy-selector: auth client not available")
	}
	return NewRemoteVerifyStrategy(s.authClient, s.cfg), nil
}

// FallbackStrategy 显式获取降级策略。
func (s *StrategySelector) FallbackStrategy() (*FallbackVerifyStrategy, error) {
	local, err := s.LocalStrategy()
	if err != nil {
		return nil, err
	}
	remote, err := s.RemoteStrategy()
	if err != nil {
		return nil, err
	}
	return NewFallbackVerifyStrategy(local, remote), nil
}
