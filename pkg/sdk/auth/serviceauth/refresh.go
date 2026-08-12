package serviceauth

import (
	"context"
	"fmt"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (h *ServiceAuthHelper) refreshTokenWithRetry(ctx context.Context) error {
	if RefreshState(h.state.Load()) == RefreshStateCircuitOpen {
		h.mu.RLock()
		openUntil := h.circuitOpenUntil
		h.mu.RUnlock()

		if time.Now().Before(openUntil) {
			return fmt.Errorf("service_auth: circuit breaker open until %v", openUntil)
		}

		h.closeCircuit()
	}

	h.stats.totalRefreshes.Add(1)
	if err := h.refreshToken(ctx); err != nil {
		return h.handleRefreshFailure(err)
	}
	return h.handleRefreshSuccess()
}

func (h *ServiceAuthHelper) handleRefreshSuccess() error {
	h.stats.successfulRefreshes.Add(1)
	h.stats.lastRefreshTime.Store(time.Now())
	h.consecutiveFailures.Store(0)
	h.state.Store(int32(RefreshStateNormal))

	if h.strategy.OnRefreshSuccess != nil {
		h.mu.RLock()
		token := h.currentToken
		expiresAt := h.expiresAt
		h.mu.RUnlock()
		h.strategy.OnRefreshSuccess(token, time.Until(expiresAt))
	}

	return nil
}

func (h *ServiceAuthHelper) handleRefreshFailure(err error) error {
	h.stats.failedRefreshes.Add(1)
	h.stats.lastRefreshError.Store(err)
	failures := h.consecutiveFailures.Add(1)

	nextRetry := h.calculateBackoff(int(failures))
	if h.strategy.OnRefreshFailure != nil {
		h.strategy.OnRefreshFailure(err, int(failures), nextRetry)
	}

	if int(failures) >= h.strategy.MaxRetries {
		h.openCircuit()
		return fmt.Errorf("service_auth: circuit breaker opened after %d failures: %w", failures, err)
	}

	h.state.Store(int32(RefreshStateRetrying))
	return err
}

func (h *ServiceAuthHelper) openCircuit() {
	h.state.Store(int32(RefreshStateCircuitOpen))

	h.mu.Lock()
	h.circuitOpenUntil = time.Now().Add(h.strategy.CircuitOpenDuration)
	h.mu.Unlock()

	if h.strategy.OnCircuitOpen != nil {
		h.strategy.OnCircuitOpen()
	}
}

func (h *ServiceAuthHelper) closeCircuit() {
	oldState := RefreshState(h.state.Swap(int32(RefreshStateNormal)))
	if oldState == RefreshStateCircuitOpen {
		h.consecutiveFailures.Store(0)
		if h.strategy.OnCircuitClose != nil {
			h.strategy.OnCircuitClose()
		}
	}
}

func (h *ServiceAuthHelper) refreshToken(ctx context.Context) error {
	req := &authnv2.IssueServiceTokenRequest{
		Subject:  h.config.ServiceID,
		Audience: h.config.TargetAudience,
	}
	if h.config.TokenTTL > 0 {
		req.Ttl = durationpb.New(h.config.TokenTTL)
	}

	resp, err := h.authClient.IssueServiceToken(ctx, req)
	if err != nil {
		return err
	}

	tokenPair := resp.TokenPair
	if tokenPair == nil {
		return fmt.Errorf("service_auth: empty token pair in response")
	}

	expiresIn := tokenPair.ExpiresIn.AsDuration()

	h.mu.Lock()
	h.currentToken = tokenPair.AccessToken
	h.expiresAt = time.Now().Add(expiresIn)
	h.mu.Unlock()

	return nil
}
