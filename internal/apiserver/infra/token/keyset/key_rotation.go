package keyset

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// KeyRotation 密钥轮换服务
// 实现 Rotator 接口
type KeyRotation struct {
	keyRepo Repository
	keyGen  KeyGenerator
	policy  RotationPolicy
	logger  log.Logger
}

// NewKeyRotation 创建密钥轮换服务
func NewKeyRotation(
	keyRepo Repository,
	keyGen KeyGenerator,
	policy RotationPolicy,
	logger log.Logger,
) *KeyRotation {
	return &KeyRotation{
		keyRepo: keyRepo,
		keyGen:  keyGen,
		policy:  policy,
		logger:  logger,
	}
}

// RotateKey 执行密钥轮换
// 轮换流程：
// 1. 生成新密钥（Active 状态）
// 2. 将当前 Active 密钥转为 Grace 状态
// 3. 清理超过 MaxKeys 的密钥（将最老的 Grace 密钥转为 Retired）
// 4. 清理过期的 Retired 密钥
func (s *KeyRotation) RotateKey(ctx context.Context) (*Key, error) {
	s.logger.Info("Starting key rotation")

	// Step 1: 将当前所有 Active 密钥转为 Grace 状态
	activeKeys, err := s.keyRepo.FindByStatus(ctx, KeyActive)
	if err != nil {
		s.logger.Errorw("Failed to find active keys", "error", err)
		return nil, errors.WithCode(code.ErrDatabase, "failed to find active keys: %v", err)
	}

	for _, key := range activeKeys {
		if err := key.EnterGrace(); err != nil {
			s.logger.Errorw("Failed to enter grace period", "kid", key.Kid, "error", err)
			return nil, err
		}
		if err := s.keyRepo.Update(ctx, key); err != nil {
			s.logger.Errorw("Failed to update key to grace", "kid", key.Kid, "error", err)
			return nil, errors.WithCode(code.ErrDatabase, "failed to update key: %v", err)
		}
		s.logger.Infow("Moved active key to grace period", "kid", key.Kid)
	}

	// Step 2: 生成新密钥（Active 状态）
	algorithm := "RS256" // 默认算法
	kid := fmt.Sprintf("key-%d", time.Now().Unix())

	keyPair, err := s.keyGen.GenerateKeyPair(ctx, algorithm, kid)
	if err != nil {
		s.logger.Errorw("Failed to generate new key", "error", err)
		return nil, errors.WithCode(code.ErrDatabase, "failed to generate key: %v", err)
	}

	// 设置密钥生效时间和过期时间
	now := time.Now()
	notBefore := now
	notAfter := now.Add(s.policy.RotationInterval + s.policy.GracePeriod)

	newKey := NewKey(
		kid,
		keyPair.PublicJWK,
		WithNotBefore(notBefore),
		WithNotAfter(notAfter),
		WithStatus(KeyActive),
	)

	if err := s.keyRepo.Save(ctx, newKey); err != nil {
		s.logger.Errorw("Failed to save new key", "kid", kid, "error", err)
		return nil, errors.WithCode(code.ErrDatabase, "failed to save key: %v", err)
	}

	s.logger.Infow("New key generated and activated",
		"kid", kid,
		"algorithm", algorithm,
		"notBefore", notBefore,
		"notAfter", notAfter,
	)

	// Step 3: 清理超过 MaxKeys 的密钥
	if err := s.cleanupExcessKeys(ctx); err != nil {
		s.logger.Warnw("Failed to cleanup excess keys", "error", err)
		// 不返回错误，继续执行
	}

	// Step 4: 清理过期的 Retired 密钥
	deleted, err := s.cleanupExpiredRetiredKeys(ctx)
	if err != nil {
		s.logger.Warnw("Failed to cleanup expired keys", "error", err)
		// 不返回错误，继续执行
	} else if deleted > 0 {
		s.logger.Infow("Cleaned up expired keys", "count", deleted)
	}

	s.logger.Info("Key rotation completed successfully")
	return newKey, nil
}

// ShouldRotate 判断是否需要轮换
// 根据 RotationPolicy 判断当前 Active 密钥是否已到轮换时间
func (s *KeyRotation) ShouldRotate(ctx context.Context) (bool, error) {
	// 获取当前 Active 密钥
	activeKeys, err := s.keyRepo.FindByStatus(ctx, KeyActive)
	if err != nil {
		return false, errors.WithCode(code.ErrDatabase, "failed to find active keys: %v", err)
	}

	// 如果没有 Active 密钥，需要立即轮换（创建第一个密钥）
	if len(activeKeys) == 0 {
		s.logger.Info("No active keys found, rotation needed")
		return true, nil
	}

	// 检查最老的 Active 密钥是否已到轮换时间
	now := time.Now()
	for _, key := range activeKeys {
		if key.NotBefore != nil {
			age := now.Sub(*key.NotBefore)
			if age >= s.policy.RotationInterval {
				s.logger.Infow("Key rotation needed",
					"kid", key.Kid,
					"age", age,
					"rotationInterval", s.policy.RotationInterval,
				)
				return true, nil
			}
		}
	}

	return false, nil
}

// GetRotationPolicy 获取当前轮换策略
func (s *KeyRotation) GetRotationPolicy() RotationPolicy {
	return s.policy
}

// UpdateRotationPolicy 更新轮换策略
func (s *KeyRotation) UpdateRotationPolicy(ctx context.Context, policy RotationPolicy) error {
	// 验证策略有效性
	if err := policy.Validate(); err != nil {
		return err
	}

	s.policy = policy
	s.logger.Infow("Rotation policy updated",
		"rotationInterval", policy.RotationInterval,
		"gracePeriod", policy.GracePeriod,
		"maxKeysInJWKS", policy.MaxKeysInJWKS,
	)

	return nil
}

// GetRotationStatus 获取轮换状态
func (s *KeyRotation) GetRotationStatus(ctx context.Context) (*RotationStatus, error) {
	// 获取 Active 密钥
	activeKeys, err := s.keyRepo.FindByStatus(ctx, KeyActive)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find active keys: %v", err)
	}

	// 获取 Grace 密钥
	graceKeys, err := s.keyRepo.FindByStatus(ctx, KeyGrace)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find grace keys: %v", err)
	}

	// 获取 Retired 密钥数量
	retiredCount, err := s.keyRepo.CountByStatus(ctx, KeyRetired)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to count retired keys: %v", err)
	}

	status := &RotationStatus{
		Policy:      s.policy,
		RetiredKeys: int(retiredCount),
	}

	// 设置 Active 密钥信息
	if len(activeKeys) > 0 {
		key := activeKeys[0]
		status.ActiveKey = &KeyInfo{
			Kid:       key.Kid,
			Status:    key.Status,
			Algorithm: key.JWK.Alg,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			CreatedAt: key.CreatedAt,
		}

		// 计算上次轮换时间和下次轮换时间
		if key.NotBefore != nil {
			status.LastRotation = *key.NotBefore
			status.NextRotation = key.NotBefore.Add(s.policy.RotationInterval)
		}
	}

	// 设置 Grace 密钥列表
	for _, key := range graceKeys {
		status.GraceKeys = append(status.GraceKeys, &KeyInfo{
			Kid:       key.Kid,
			Status:    key.Status,
			Algorithm: key.JWK.Alg,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			CreatedAt: key.CreatedAt,
		})
	}

	return status, nil
}

// cleanupExcessKeys 清理超过 MaxKeys 的密钥
// 将最老的 Grace 密钥转为 Retired
func (s *KeyRotation) cleanupExcessKeys(ctx context.Context) error {
	// 计算当前 JWKS 中的密钥数量（Active + Grace）
	activeCount, err := s.keyRepo.CountByStatus(ctx, KeyActive)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to count active keys: %v", err)
	}

	graceCount, err := s.keyRepo.CountByStatus(ctx, KeyGrace)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to count grace keys: %v", err)
	}

	totalKeys := int(activeCount + graceCount)

	// 如果密钥数量未超过限制，不需要清理
	if totalKeys <= s.policy.MaxKeysInJWKS {
		return nil
	}

	// 获取所有 Grace 密钥（按创建时间排序）
	graceKeys, err := s.keyRepo.FindByStatus(ctx, KeyGrace)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to find grace keys: %v", err)
	}

	// 计算需要退役的密钥数量
	needToRetire := totalKeys - s.policy.MaxKeysInJWKS

	// 将最老的 Grace 密钥转为 Retired
	for i := 0; i < needToRetire && i < len(graceKeys); i++ {
		key := graceKeys[i]
		if err := key.Retire(); err != nil {
			s.logger.Warnw("Failed to retire grace key", "kid", key.Kid, "error", err)
			continue
		}
		if err := s.keyRepo.Update(ctx, key); err != nil {
			s.logger.Warnw("Failed to update retired key", "kid", key.Kid, "error", err)
			continue
		}
		s.logger.Infow("Grace key retired due to excess", "kid", key.Kid)
	}

	return nil
}

// cleanupExpiredRetiredKeys 清理过期的 Retired 密钥
// 删除 NotAfter < now 且 Status = Retired 的密钥
func (s *KeyRotation) cleanupExpiredRetiredKeys(ctx context.Context) (int, error) {
	retiredKeys, err := s.keyRepo.FindByStatus(ctx, KeyRetired)
	if err != nil {
		return 0, errors.WithCode(code.ErrDatabase, "failed to find retired keys: %v", err)
	}

	now := time.Now()
	deleted := 0

	for _, key := range retiredKeys {
		// 只删除已过期的密钥
		if key.NotAfter != nil && now.After(*key.NotAfter) {
			if err := s.keyRepo.Delete(ctx, key.Kid); err != nil {
				s.logger.Warnw("Failed to delete expired key", "kid", key.Kid, "error", err)
				continue
			}
			s.logger.Infow("Deleted expired retired key", "kid", key.Kid, "notAfter", *key.NotAfter)
			deleted++
		}
	}

	return deleted, nil
}
