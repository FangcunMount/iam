package serviceauth

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/pkg/sdk/config"
)

// NewServiceAuthHelper 创建服务认证助手。
func NewServiceAuthHelper(cfg *config.ServiceAuthConfig, authClient ServiceTokenIssuer, opts ...ServiceAuthOption) (*ServiceAuthHelper, error) {
	h := &ServiceAuthHelper{
		config:     cfg,
		authClient: authClient,
		strategy:   DefaultRefreshStrategy(),
		stopCh:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(h)
	}

	if err := h.refreshTokenWithRetry(context.Background()); err != nil {
		return nil, err
	}

	go h.refreshLoop()
	return h, nil
}

// NewServiceAuthHelperWithCallbacks 创建带回调的服务认证助手。
func NewServiceAuthHelperWithCallbacks(
	cfg *config.ServiceAuthConfig,
	authClient ServiceTokenIssuer,
	onSuccess func(token string, expiresIn time.Duration),
	onFailure func(err error, attempt int, nextRetry time.Duration),
) (*ServiceAuthHelper, error) {
	strategy := DefaultRefreshStrategy()
	strategy.OnRefreshSuccess = onSuccess
	strategy.OnRefreshFailure = onFailure

	return NewServiceAuthHelper(cfg, authClient, WithRefreshStrategy(strategy))
}
