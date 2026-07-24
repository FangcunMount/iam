package keyset

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// KeyRotation coordinates automatic rotation through the atomic KeyManager.
type KeyRotation struct {
	manager *KeyManager
	policy  RotationPolicy
	logger  log.Logger
}

func NewKeyRotation(manager *KeyManager, policy RotationPolicy, logger log.Logger) *KeyRotation {
	return &KeyRotation{manager: manager, policy: policy, logger: logger}
}

// RotateKey performs an explicit administrator-requested rotation.
func (s *KeyRotation) RotateKey(ctx context.Context) (*Key, error) {
	key, err := s.manager.CreateKey(ctx, "RS256", nil, nil)
	if err != nil {
		recordRotationResult("failed")
		s.logger.Errorw("explicit jwks rotation failed", "error", err)
		return nil, err
	}
	s.afterRotation(ctx, true)
	return key, nil
}

// RotateIfDue atomically rechecks the policy inside the repository transition.
func (s *KeyRotation) RotateIfDue(ctx context.Context) (*Key, bool, error) {
	key, rotated, err := s.manager.RotateIfDue(ctx, "RS256")
	if err != nil {
		recordRotationResult("failed")
		s.logger.Errorw("scheduled jwks rotation failed", "error", err)
		return nil, false, err
	}
	s.afterRotation(ctx, rotated)
	return key, rotated, nil
}

func (s *KeyRotation) afterRotation(ctx context.Context, rotated bool) {
	if !rotated {
		recordRotationResult("noop")
		s.logger.Debugw("jwks rotation check completed", "result", "noop")
		return
	}
	if _, err := s.manager.CleanupExpiredKeys(ctx); err != nil {
		s.logger.Warnw("jwks expired-key cleanup failed after rotation", "error", err)
	}
	active, _ := s.manager.keyRepo.CountByStatus(ctx, KeyActive)
	grace, _ := s.manager.keyRepo.CountByStatus(ctx, KeyGrace)
	retired, _ := s.manager.keyRepo.CountByStatus(ctx, KeyRetired)
	setKeyStateCounts(active, grace, retired)
	if int(active+grace) > s.policy.MaxKeysInJWKS {
		s.logger.Warnw("jwks publishable key count exceeds soft limit",
			"active", active,
			"grace", grace,
			"retired", retired,
			"maxPublishableKeys", s.policy.MaxKeysInJWKS,
		)
	}
	recordRotationResult("success")
	s.logger.Infow("jwks rotation completed", "result", "success", "active", active, "grace", grace, "retired", retired)
}

func (s *KeyRotation) ShouldRotate(ctx context.Context) (bool, error) {
	activeKeys, err := s.manager.keyRepo.FindByStatus(ctx, KeyActive)
	if err != nil {
		return false, errors.WithCode(code.ErrDatabase, "failed to find active keys: %v", err)
	}
	if len(activeKeys) == 0 {
		return true, nil
	}
	if len(activeKeys) != 1 {
		return false, errors.WithCode(code.ErrInvalidStateTransition, "expected exactly one active key, found %d", len(activeKeys))
	}
	active := activeKeys[0]
	now := s.manager.now()
	if active.IsExpired(now) {
		return true, nil
	}
	return active.NotBefore == nil || !active.NotBefore.After(now.Add(-s.policy.RotationInterval)), nil
}

func (s *KeyRotation) GetRotationPolicy() RotationPolicy {
	return s.policy
}

func (s *KeyRotation) UpdateRotationPolicy(_ context.Context, policy RotationPolicy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if err := s.manager.SetRotationPolicy(policy); err != nil {
		return err
	}
	s.policy = policy
	return nil
}

func (s *KeyRotation) GetRotationStatus(ctx context.Context) (*RotationStatus, error) {
	activeKeys, err := s.manager.keyRepo.FindByStatus(ctx, KeyActive)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find active keys: %v", err)
	}
	graceKeys, err := s.manager.keyRepo.FindByStatus(ctx, KeyGrace)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find grace keys: %v", err)
	}
	retiredCount, err := s.manager.keyRepo.CountByStatus(ctx, KeyRetired)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to count retired keys: %v", err)
	}
	status := &RotationStatus{Policy: s.policy, RetiredKeys: int(retiredCount)}
	if len(activeKeys) == 1 {
		status.ActiveKey = rotationKeyInfo(activeKeys[0])
		if activeKeys[0].NotBefore != nil {
			status.LastRotation = *activeKeys[0].NotBefore
			status.NextRotation = activeKeys[0].NotBefore.Add(s.policy.RotationInterval)
		}
	}
	for _, key := range graceKeys {
		status.GraceKeys = append(status.GraceKeys, rotationKeyInfo(key))
	}
	return status, nil
}

func rotationKeyInfo(key *Key) *KeyInfo {
	return &KeyInfo{
		Kid:       key.Kid,
		Status:    key.Status,
		Algorithm: key.JWK.Alg,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		CreatedAt: key.CreatedAt,
	}
}
