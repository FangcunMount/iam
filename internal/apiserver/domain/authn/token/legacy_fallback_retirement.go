package token

import (
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// LegacyFallbackRetirementPolicy defines the minimum evidence window required
// before deleting legacy Refresh/JWT compatibility branches.
type LegacyFallbackRetirementPolicy struct {
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	sessionMaxTTL   time.Duration
}

// LegacyFallbackObservation is the operational evidence used by the policy.
// AllFallbackMetricsZeroSince must be reset whenever any governed fallback
// counter increases.
type LegacyFallbackObservation struct {
	AllInstancesCurrentSince    time.Time
	AllFallbackMetricsZeroSince time.Time
}

func NewLegacyFallbackRetirementPolicy(
	accessTokenTTL, refreshTokenTTL, sessionMaxTTL time.Duration,
) LegacyFallbackRetirementPolicy {
	return LegacyFallbackRetirementPolicy{
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		sessionMaxTTL:   sessionMaxTTL,
	}
}

// RequiredZeroWindow returns the longest configured lifetime that may retain a
// legacy access token, refresh token, or session snapshot.
func (p LegacyFallbackRetirementPolicy) RequiredZeroWindow() (time.Duration, error) {
	if p.accessTokenTTL <= 0 || p.refreshTokenTTL <= 0 || p.sessionMaxTTL <= 0 {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "access, refresh and session TTLs must be positive")
	}
	window := max(p.accessTokenTTL, p.refreshTokenTTL)
	return max(window, p.sessionMaxTTL), nil
}

// CanRetireAt requires both a fully upgraded fleet and continuously quiet
// fallback metrics for one complete maximum-lifetime window.
func (p LegacyFallbackRetirementPolicy) CanRetireAt(
	now time.Time,
	observation LegacyFallbackObservation,
) (bool, error) {
	window, err := p.RequiredZeroWindow()
	if err != nil {
		return false, err
	}
	if observation.AllInstancesCurrentSince.IsZero() || observation.AllFallbackMetricsZeroSince.IsZero() {
		return false, nil
	}
	start := observation.AllInstancesCurrentSince
	if observation.AllFallbackMetricsZeroSince.After(start) {
		start = observation.AllFallbackMetricsZeroSince
	}
	if now.Before(start) {
		return false, nil
	}
	return now.Sub(start) >= window, nil
}
