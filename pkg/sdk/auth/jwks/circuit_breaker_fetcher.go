package jwks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// CircuitBreakerFetcher 为下游 fetcher 添加熔断保护。
type CircuitBreakerFetcher struct {
	next   KeyFetcher
	config *config.CircuitBreakerConfig

	mu           sync.RWMutex
	state        CircuitState
	failures     int
	successes    int
	lastFailTime time.Time
	stats        *FetcherStats
}

// NewCircuitBreakerFetcher 创建熔断器 Fetcher。
func NewCircuitBreakerFetcher(next KeyFetcher, cfg *config.CircuitBreakerConfig) *CircuitBreakerFetcher {
	if cfg == nil {
		cfg = &config.CircuitBreakerConfig{
			FailureThreshold: 5,
			OpenDuration:     30 * time.Second,
			HalfOpenRequests: 3,
			SuccessThreshold: 2,
		}
	}
	return &CircuitBreakerFetcher{
		next:   next,
		config: cfg,
		state:  CircuitClosed,
		stats:  &FetcherStats{},
	}
}

func (f *CircuitBreakerFetcher) Name() string {
	return "circuit-breaker"
}

func (f *CircuitBreakerFetcher) Fetch(ctx context.Context) (jwk.Set, error) {
	f.stats.IncrAttempts()

	if !f.shouldAllow() {
		f.stats.IncrFailures()
		return nil, fmt.Errorf("circuit-breaker: circuit is open")
	}

	keySet, err := f.next.Fetch(ctx)
	if err != nil {
		f.recordFailure()
		f.stats.IncrFailures()
		return nil, err
	}

	f.recordSuccess()
	f.stats.IncrSuccesses()
	return keySet, nil
}

func (f *CircuitBreakerFetcher) shouldAllow() bool {
	f.mu.RLock()
	state := f.state
	lastFail := f.lastFailTime
	f.mu.RUnlock()

	switch state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(lastFail) > f.config.OpenDuration {
			f.mu.Lock()
			f.state = CircuitHalfOpen
			f.successes = 0
			f.mu.Unlock()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return true
	}
}

func (f *CircuitBreakerFetcher) recordFailure() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failures++
	f.lastFailTime = time.Now()

	switch f.state {
	case CircuitClosed:
		if f.failures >= f.config.FailureThreshold {
			f.state = CircuitOpen
		}
	case CircuitHalfOpen:
		f.state = CircuitOpen
		f.failures = 0
	}
}

func (f *CircuitBreakerFetcher) recordSuccess() {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch f.state {
	case CircuitClosed:
		f.failures = 0
	case CircuitHalfOpen:
		f.successes++
		if f.successes >= f.config.SuccessThreshold {
			f.state = CircuitClosed
			f.failures = 0
		}
	}
}

// State 返回当前熔断器状态。
func (f *CircuitBreakerFetcher) State() CircuitState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

// Stats 返回统计信息。
func (f *CircuitBreakerFetcher) Stats() FetcherStats {
	return f.stats.Snapshot()
}
