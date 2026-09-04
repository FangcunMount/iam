package keyset

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	pkgauth "github.com/FangcunMount/iam/v3/pkg/auth"
)

// KeyRotation is the canonical keyset lifecycle mutation boundary.
type KeyRotation struct {
	manager *KeyManager
	policy  RotationPolicy
	logger  log.Logger
}

func NewKeyRotation(manager *KeyManager, policy RotationPolicy, logger log.Logger) *KeyRotation {
	return &KeyRotation{manager: manager, policy: policy, logger: logger}
}

var _ Lifecycle = (*KeyRotation)(nil)

func (s *KeyRotation) CreateAndActivate(
	ctx context.Context,
	alg string,
	notBefore, notAfter *time.Time,
) (*Key, bool, error) {
	key, err := s.manager.CreateKey(ctx, alg, notBefore, notAfter)
	if err != nil {
		recordRotationResult("failed")
		return nil, false, err
	}
	s.afterActivation(ctx, key, false)
	return key, true, nil
}

func (s *KeyRotation) Bootstrap(ctx context.Context, alg string) (*Key, bool, error) {
	key, activated, err := s.manager.BootstrapKey(ctx, alg)
	if err != nil {
		return nil, false, err
	}
	if activated {
		s.afterActivation(ctx, key, false)
	}
	return key, activated, nil
}

func (s *KeyRotation) RotateIfDue(ctx context.Context) (*Key, bool, error) {
	key, rotated, err := s.manager.RotateIfDue(ctx, pkgauth.TokenProfileAlgorithm)
	if err != nil {
		recordRotationResult("failed")
		return nil, false, err
	}
	if !rotated {
		recordRotationResult("noop")
		s.logger.Debugw("jwks lifecycle operation completed", "operation", "auto_rotate", "result", "noop", "automatic", true)
		return key, false, nil
	}
	s.afterActivation(ctx, key, true)
	return key, true, nil
}

func (s *KeyRotation) RetireKey(ctx context.Context, kid string) error {
	if err := s.manager.RetireKey(ctx, kid); err != nil {
		return err
	}
	_ = s.RefreshStateMetrics(ctx)
	return nil
}

func (s *KeyRotation) ForceRetireKey(ctx context.Context, kid string) error {
	if err := s.manager.ForceRetireKey(ctx, kid); err != nil {
		return err
	}
	_ = s.RefreshStateMetrics(ctx)
	return nil
}

func (s *KeyRotation) EnterGracePeriod(ctx context.Context, kid string) error {
	if err := s.manager.EnterGracePeriod(ctx, kid); err != nil {
		return err
	}
	_ = s.RefreshStateMetrics(ctx)
	return nil
}

func (s *KeyRotation) CleanupExpiredKeys(ctx context.Context) (int, error) {
	count, err := s.manager.CleanupExpiredKeys(ctx)
	if err != nil {
		return 0, err
	}
	_ = s.RefreshStateMetrics(ctx)
	return count, nil
}

func (s *KeyRotation) afterActivation(ctx context.Context, key *Key, automatic bool) {
	if _, err := s.manager.CleanupExpiredKeys(ctx); err != nil {
		recordPostCommitFailure("cleanup")
		s.logger.Warnw("jwks post-commit action failed", "stage", "cleanup")
	}
	active, grace, retired, err := s.refreshStateMetrics(ctx)
	if err == nil && int(active+grace) > s.policy.MaxKeysInJWKS {
		s.logger.Warnw("jwks publishable key count exceeds soft limit",
			"active", active,
			"grace", grace,
			"retired", retired,
			"maxPublishableKeys", s.policy.MaxKeysInJWKS,
		)
	}
	recordRotationResult("success")
	operation := "create_and_activate"
	if automatic {
		operation = "auto_rotate"
	}
	s.logger.Infow("jwks lifecycle operation completed",
		"operation", operation,
		"result", "success",
		"kid", key.Kid,
		"automatic", automatic,
	)
}

// RefreshStateMetrics initializes or refreshes lifecycle state gauges.
func (s *KeyRotation) RefreshStateMetrics(ctx context.Context) error {
	_, _, _, err := s.refreshStateMetrics(ctx)
	return err
}

func (s *KeyRotation) refreshStateMetrics(ctx context.Context) (int64, int64, int64, error) {
	stats, err := s.manager.GetKeyStats(ctx)
	if err != nil {
		recordPostCommitFailure("state_count")
		s.logger.Warnw("jwks post-commit action failed", "stage", "state_count")
		return 0, 0, 0, err
	}
	active := stats[KeyActive]
	grace := stats[KeyGrace]
	retired := stats[KeyRetired]
	setKeyStateCounts(active, grace, retired)
	return active, grace, retired, nil
}
