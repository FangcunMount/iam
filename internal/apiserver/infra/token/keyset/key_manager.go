package keyset

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	pkgauth "github.com/FangcunMount/iam/v3/pkg/auth"
	"github.com/google/uuid"
)

// KeyManager 密钥生命周期管理服务
// 实现 Manager 接口
type KeyManager struct {
	keyRepo      Repository
	keyGenerator KeyGenerator
	privateStore PrivateKeyStorage
	policy       RotationPolicy
	now          func() time.Time
}

// NewKeyManager 创建密钥管理器
func NewKeyManager(
	keyRepo Repository,
	keyGenerator KeyGenerator,
	privateStore ...PrivateKeyStorage,
) *KeyManager {
	var store PrivateKeyStorage
	if len(privateStore) > 0 {
		store = privateStore[0]
	}
	return &KeyManager{
		keyRepo:      keyRepo,
		keyGenerator: keyGenerator,
		privateStore: store,
		policy:       DefaultRotationPolicy(),
		now:          time.Now,
	}
}

// NewKeyManagerWithPolicy creates a lifecycle manager with explicit storage and policy.
func NewKeyManagerWithPolicy(
	keyRepo Repository,
	keyGenerator KeyGenerator,
	privateStore PrivateKeyStorage,
	policy RotationPolicy,
) *KeyManager {
	manager := NewKeyManager(keyRepo, keyGenerator, privateStore)
	manager.policy = policy
	return manager
}

// Ensure KeyManager implements KeyManagementService
var _ Manager = (*KeyManager)(nil)

// CreateKey 创建新密钥
func (s *KeyManager) CreateKey(
	ctx context.Context,
	alg string,
	notBefore, notAfter *time.Time,
) (*Key, error) {
	result, err := s.createAndActivate(ctx, alg, notBefore, notAfter, activationModeForce)
	return result.Active, err
}

type activationMode uint8

const (
	activationModeForce activationMode = iota + 1
	activationModeBootstrap
	activationModeIfDue
)

type keyActivationResult struct {
	Active    *Key
	Activated bool
}

func (s *KeyManager) BootstrapKey(ctx context.Context, alg string) (*Key, bool, error) {
	result, err := s.createAndActivate(ctx, alg, nil, nil, activationModeBootstrap)
	return result.Active, result.Activated, err
}

func (s *KeyManager) RotateIfDue(ctx context.Context, alg string) (*Key, bool, error) {
	result, err := s.createAndActivate(ctx, alg, nil, nil, activationModeIfDue)
	return result.Active, result.Activated, err
}

func (s *KeyManager) createAndActivate(
	ctx context.Context,
	alg string,
	notBefore, notAfter *time.Time,
	mode activationMode,
) (keyActivationResult, error) {
	if alg != pkgauth.TokenProfileAlgorithm {
		return keyActivationResult{}, errors.WithCode(code.ErrInvalidJWKAlg, "algorithm must be %s", pkgauth.TokenProfileAlgorithm)
	}
	activator, ok := s.keyRepo.(AtomicActivator)
	if !ok {
		return keyActivationResult{}, errors.WithCode(code.ErrDatabase, "jwks repository does not support atomic activation")
	}
	if s.keyGenerator == nil {
		return keyActivationResult{}, errors.WithCode(code.ErrUnknown, "jwks key generator is not configured")
	}

	now := s.now()
	effectiveNotBefore := now
	if notBefore != nil {
		effectiveNotBefore = *notBefore
	}
	if effectiveNotBefore.After(now) {
		return keyActivationResult{}, errors.WithCode(code.ErrInvalidTimeRange, "NotBefore cannot be in the future for an active key")
	}
	effectiveNotAfter := now.Add(s.policy.RotationInterval + s.policy.GracePeriod)
	if notAfter != nil {
		effectiveNotAfter = *notAfter
	}
	if !effectiveNotAfter.After(effectiveNotBefore) {
		return keyActivationResult{}, errors.WithCode(code.ErrInvalidTimeRange, "NotAfter must be after NotBefore")
	}

	kid := "key-" + uuid.NewString()
	keyPair, err := s.keyGenerator.GenerateKeyPair(ctx, alg, kid)
	if err != nil {
		return keyActivationResult{}, errors.WithCode(code.ErrUnknown, "failed to generate key pair: %v", err)
	}
	candidate := NewKey(
		kid,
		keyPair.PublicJWK,
		WithNotBefore(effectiveNotBefore),
		WithNotAfter(effectiveNotAfter),
		WithStatus(KeyActive),
	)
	if err := candidate.Validate(); err != nil {
		return keyActivationResult{}, err
	}
	if s.privateStore != nil {
		if err := s.privateStore.SavePrivateKey(ctx, kid, keyPair.PrivateKey, alg); err != nil {
			return keyActivationResult{}, err
		}
	}

	request := ActivationRequest{
		Candidate:  candidate,
		Now:        now,
		GraceUntil: now.Add(s.policy.GracePeriod),
	}
	switch mode {
	case activationModeBootstrap:
		request.RequireNoActive = true
	case activationModeIfDue:
		dueBefore := now.Add(-s.policy.RotationInterval)
		request.DueBefore = &dueBefore
	}
	activation, err := activator.Activate(ctx, request)
	if err != nil {
		s.deleteCandidatePEM(ctx, kid)
		if errors.IsCode(err, code.ErrKeyAlreadyExists) && mode != activationModeForce {
			if active, getErr := s.GetActiveKey(ctx); getErr == nil {
				return keyActivationResult{Active: active}, nil
			}
		}
		return keyActivationResult{}, errors.WithCode(code.ErrDatabase, "failed to atomically activate key: %v", err)
	}
	if !activation.Activated {
		s.deleteCandidatePEM(ctx, kid)
	}
	return keyActivationResult{Active: activation.Active, Activated: activation.Activated}, nil
}

func (s *KeyManager) deleteCandidatePEM(ctx context.Context, kid string) {
	if s.privateStore == nil {
		return
	}
	if err := s.privateStore.DeletePrivateKey(ctx, kid); err != nil && !errors.IsCode(err, code.ErrKeyNotFound) {
		candidateCleanupFailures.Inc()
		log.Warnw("failed to remove unused jwks candidate", "kid", kid)
	}
}

// GetActiveKey 获取当前激活的密钥
func (s *KeyManager) GetActiveKey(ctx context.Context) (*Key, error) {
	keys, err := s.keyRepo.FindByStatus(ctx, KeyActive)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find active keys: %v", err)
	}

	if len(keys) == 0 {
		return nil, errors.WithCode(code.ErrNoActiveKey, "no active key available")
	}
	if len(keys) != 1 {
		return nil, errors.WithCode(code.ErrInvalidStateTransition, "expected exactly one active key, found %d", len(keys))
	}

	// 过滤出可以用于签名的密钥（未过期且状态正确）
	now := s.now()
	for _, key := range keys {
		if key.CanSign() && key.IsValidAt(now) {
			return key, nil
		}
	}

	return nil, errors.WithCode(code.ErrNoActiveKey, "no valid active key available")
}

// ValidateActiveKey verifies that the only active row is currently valid and
// that its PEM private key matches the public JWK stored in MySQL.
func (s *KeyManager) ValidateActiveKey(ctx context.Context, resolver PrivateKeyResolver) (*Key, error) {
	active, err := s.GetActiveKey(ctx)
	if err != nil {
		return nil, err
	}
	if resolver == nil {
		return nil, errors.WithCode(code.ErrUnknown, "private key resolver is not configured")
	}
	privateKey, err := resolver.ResolveSigningKey(ctx, active.Kid, active.JWK.Alg)
	if err != nil {
		materialValidationFailures.WithLabelValues("resolve").Inc()
		return nil, errors.WithCode(code.ErrUnknown, "resolve active private key %s: %v", active.Kid, err)
	}
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		materialValidationFailures.WithLabelValues("type").Inc()
		return nil, errors.WithCode(code.ErrInvalidJWK, "active private key %s is not RSA", active.Kid)
	}
	expectedN := base64.RawURLEncoding.EncodeToString(rsaKey.PublicKey.N.Bytes())
	expectedE := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.PublicKey.E)).Bytes())
	if active.JWK.N == nil || active.JWK.E == nil || *active.JWK.N != expectedN || *active.JWK.E != expectedE {
		materialValidationFailures.WithLabelValues("mismatch").Inc()
		return nil, errors.WithCode(code.ErrInvalidJWK, "active private key does not match public JWK for kid %s", active.Kid)
	}
	return active, nil
}

// GetKeyByKid 根据 kid 获取密钥
func (s *KeyManager) GetKeyByKid(ctx context.Context, kid string) (*Key, error) {
	key, err := s.keyRepo.FindByKid(ctx, kid)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find key: %v", err)
	}

	if key == nil {
		return nil, errors.WithCode(code.ErrKeyNotFound, "key not found: %s", kid)
	}

	return key, nil
}

// RetireKey 退役密钥（Grace → Retired）
func (s *KeyManager) RetireKey(ctx context.Context, kid string) error {
	key, err := s.keyRepo.FindByKid(ctx, kid)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to find key: %v", err)
	}

	if key == nil {
		return errors.WithCode(code.ErrKeyNotFound, "key not found: %s", kid)
	}
	if !key.IsExpired(s.now()) {
		return errors.WithCode(code.ErrInvalidStateTransition, "grace key cannot be retired before NotAfter")
	}

	// 状态转换（Grace → Retired）
	if err := key.Retire(); err != nil {
		return err
	}

	// 保存状态
	if err := s.keyRepo.Update(ctx, key); err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to update key: %v", err)
	}

	return nil
}

// ForceRetireKey 强制退役密钥（任何状态 → Retired）
func (s *KeyManager) ForceRetireKey(ctx context.Context, kid string) error {
	key, err := s.keyRepo.FindByKid(ctx, kid)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to find key: %v", err)
	}

	if key == nil {
		return errors.WithCode(code.ErrKeyNotFound, "key not found: %s", kid)
	}
	if key.IsActive() {
		return errors.WithCode(code.ErrInvalidStateTransition, "cannot force-retire the active signing key; activate a replacement first")
	}

	// 强制状态转换（任何状态 → Retired）
	key.ForceRetire()

	// 保存状态
	if err := s.keyRepo.Update(ctx, key); err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to update key: %v", err)
	}

	return nil
}

// EnterGracePeriod 进入宽限期（Active → Grace）
func (s *KeyManager) EnterGracePeriod(ctx context.Context, kid string) error {
	key, err := s.keyRepo.FindByKid(ctx, kid)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to find key: %v", err)
	}

	if key == nil {
		return errors.WithCode(code.ErrKeyNotFound, "key not found: %s", kid)
	}
	if key.IsActive() {
		return errors.WithCode(code.ErrInvalidStateTransition, "cannot move the only active signing key to grace; activate a replacement first")
	}

	// 状态转换（Active → Grace）
	if err := key.EnterGrace(); err != nil {
		return err
	}

	// 保存状态
	if err := s.keyRepo.Update(ctx, key); err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to update key: %v", err)
	}

	return nil
}

// CleanupExpiredKeys 清理过期密钥
// 删除 NotAfter < now 且 Status = Retired 的密钥
func (s *KeyManager) CleanupExpiredKeys(ctx context.Context) (int, error) {
	// 查询已过期的密钥
	expiredKeys, err := s.keyRepo.FindExpired(ctx)
	if err != nil {
		return 0, errors.WithCode(code.ErrDatabase, "failed to find expired keys: %v", err)
	}

	if len(expiredKeys) == 0 {
		return 0, nil
	}

	// Expired non-active keys are no longer publishable. Retire and delete the
	// database row first, then remove the private material.
	deletedCount := 0
	for _, key := range expiredKeys {
		if key.IsActive() {
			continue
		}
		if !key.IsRetired() {
			key.ForceRetire()
			if err := s.keyRepo.Update(ctx, key); err != nil {
				continue
			}
		}
		if err := s.keyRepo.Delete(ctx, key.Kid); err != nil {
			continue
		}
		deletedCount++
		if s.privateStore != nil {
			if err := s.privateStore.DeletePrivateKey(ctx, key.Kid); err != nil && !errors.IsCode(err, code.ErrKeyNotFound) {
				recordPostCommitFailure("private_key_delete")
				log.Warnw("failed to delete retired jwks private key", "kid", key.Kid)
			}
		}
	}

	return deletedCount, nil
}

// ListKeys 列出密钥（分页）
func (s *KeyManager) ListKeys(
	ctx context.Context,
	status KeyStatus,
	limit, offset int,
) ([]*Key, int64, error) {
	// 如果指定了状态，按状态查询
	if status != 0 {
		keys, err := s.keyRepo.FindByStatus(ctx, status)
		if err != nil {
			return nil, 0, errors.WithCode(code.ErrDatabase, "failed to find keys: %v", err)
		}

		// 手动分页
		total := int64(len(keys))
		start := offset
		if start > len(keys) {
			start = len(keys)
		}
		end := start + limit
		if end > len(keys) {
			end = len(keys)
		}

		return keys[start:end], total, nil
	}

	// 查询所有密钥（分页）
	keys, total, err := s.keyRepo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, errors.WithCode(code.ErrDatabase, "failed to find keys: %v", err)
	}

	return keys, total, nil
}

// GetKeyStats 获取密钥统计信息（辅助方法）
func (s *KeyManager) GetKeyStats(ctx context.Context) (map[KeyStatus]int64, error) {
	stats := make(map[KeyStatus]int64)

	for _, status := range []KeyStatus{KeyActive, KeyGrace, KeyRetired} {
		count, err := s.keyRepo.CountByStatus(ctx, status)
		if err != nil {
			return nil, errors.WithCode(code.ErrDatabase, "failed to count keys: %v", err)
		}
		stats[status] = count
	}

	return stats, nil
}

// ValidateKeyHealth 验证密钥健康状态（辅助方法）
// 检查是否有可用的 Active 密钥
func (s *KeyManager) ValidateKeyHealth(ctx context.Context) error {
	activeKey, err := s.GetActiveKey(ctx)
	if err != nil {
		return fmt.Errorf("no active key available: %w", err)
	}

	// 检查密钥是否即将过期（24小时内）
	if activeKey.NotAfter != nil {
		timeUntilExpiry := time.Until(*activeKey.NotAfter)
		if timeUntilExpiry < 24*time.Hour {
			return fmt.Errorf("active key expires in %v", timeUntilExpiry)
		}
	}

	return nil
}
