package serviceauth

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
)

// ServiceTokenIssuer 定义服务间认证所需的最小签发能力。
type ServiceTokenIssuer interface {
	IssueServiceToken(context.Context, *authnv2.IssueServiceTokenRequest) (*authnv2.IssueServiceTokenResponse, error)
}

// RefreshStrategy 刷新策略配置。
type RefreshStrategy struct {
	JitterRatio         float64
	MinBackoff          time.Duration
	MaxBackoff          time.Duration
	BackoffMultiplier   float64
	MaxRetries          int
	CircuitOpenDuration time.Duration
	OnRefreshSuccess    func(token string, expiresIn time.Duration)
	OnRefreshFailure    func(err error, attempt int, nextRetry time.Duration)
	OnCircuitOpen       func()
	OnCircuitClose      func()
}

// RefreshState 刷新状态。
type RefreshState int32

const (
	RefreshStateNormal RefreshState = iota
	RefreshStateRetrying
	RefreshStateCircuitOpen
)

// RefreshStats 刷新统计。
type RefreshStats struct {
	TotalRefreshes      int64
	SuccessfulRefreshes int64
	FailedRefreshes     int64
	ConsecutiveFailures int64
	LastRefreshTime     time.Time
	LastRefreshError    error
	State               RefreshState
}

// ServiceAuthHelper 服务间认证助手。
type ServiceAuthHelper struct {
	config     *config.ServiceAuthConfig
	authClient ServiceTokenIssuer
	strategy   *RefreshStrategy

	mu           sync.RWMutex
	currentToken string
	expiresAt    time.Time
	stopCh       chan struct{}

	state               atomic.Int32
	consecutiveFailures atomic.Int64
	circuitOpenUntil    time.Time

	stats struct {
		totalRefreshes      atomic.Int64
		successfulRefreshes atomic.Int64
		failedRefreshes     atomic.Int64
		lastRefreshTime     atomic.Value
		lastRefreshError    atomic.Value
	}
}

// ServiceAuthOption 配置选项。
type ServiceAuthOption func(*ServiceAuthHelper)
