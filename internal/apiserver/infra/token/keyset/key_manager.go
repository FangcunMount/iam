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
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	pkgauth "github.com/FangcunMount/iam/v5/pkg/auth"
	"github.com/google/uuid"
)

// KeyManager 密钥生命周期管理服务
// 实现 Manager 接口
type KeyManager struct {
	keyRepo      Repository        // 密钥仓库
	keyGenerator KeyGenerator      // 密钥生成器
	privateStore PrivateKeyStorage // 私钥存储
	policy       RotationPolicy    // 旋转策略
	now          func() time.Time  // 当前时间
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
	// 创建密钥管理器
	return &KeyManager{
		keyRepo:      keyRepo,
		keyGenerator: keyGenerator,
		privateStore: store,
		policy:       DefaultRotationPolicy(),
		now:          time.Now,
	}
}

// NewKeyManagerWithPolicy 创建密钥管理器，指定存储和策略。
func NewKeyManagerWithPolicy(
	keyRepo Repository,
	keyGenerator KeyGenerator,
	privateStore PrivateKeyStorage,
	policy RotationPolicy,
) *KeyManager {
	// 创建密钥管理器
	manager := NewKeyManager(keyRepo, keyGenerator, privateStore)
	// 设置旋转策略
	manager.policy = policy
	return manager
}

// 确保 KeyManager 实现 Manager 接口
var _ Manager = (*KeyManager)(nil)

// CreateKey 创建新密钥，并激活。
func (s *KeyManager) CreateKey(
	ctx context.Context,
	alg string,
	notBefore, notAfter *time.Time,
) (*Key, error) {
	// 创建并激活密钥
	result, err := s.createAndActivate(ctx, alg, notBefore, notAfter, activationModeForce)
	// 返回激活的密钥
	return result.Active, err
}

// activationMode 激活模式
type activationMode uint8

const (
	activationModeForce     activationMode = iota + 1 // 强制激活
	activationModeBootstrap                           // 引导激活
	activationModeIfDue                               // 如果到期则激活
)

// keyActivationResult 密钥激活结果
type keyActivationResult struct {
	Active    *Key // 活动密钥
	Activated bool // 是否激活
}

// BootstrapKey 引导激活密钥
func (s *KeyManager) BootstrapKey(ctx context.Context, alg string) (*Key, bool, error) {
	result, err := s.createAndActivate(ctx, alg, nil, nil, activationModeBootstrap)
	return result.Active, result.Activated, err
}

func (s *KeyManager) RotateIfDue(ctx context.Context, alg string) (*Key, bool, error) {
	result, err := s.createAndActivate(ctx, alg, nil, nil, activationModeIfDue)
	return result.Active, result.Activated, err
}

// createAndActivate 创建并激活密钥
func (s *KeyManager) createAndActivate(
	ctx context.Context,
	alg string,
	notBefore, notAfter *time.Time,
	mode activationMode,
) (keyActivationResult, error) {
	// 如果算法不支持，则返回错误
	if alg != pkgauth.TokenProfileAlgorithm {
		return keyActivationResult{}, errors.WithCode(code.ErrInvalidJWKAlg, "algorithm must be %s", pkgauth.TokenProfileAlgorithm)
	}
	// 如果密钥仓库不支持原子激活，则返回错误
	activator, ok := s.keyRepo.(AtomicActivator)
	if !ok {
		return keyActivationResult{}, errors.WithCode(code.ErrDatabase, "jwks repository does not support atomic activation")
	}
	// 如果密钥生成器未配置，则返回错误
	if s.keyGenerator == nil {
		return keyActivationResult{}, errors.WithCode(code.ErrUnknown, "jwks key generator is not configured")
	}

	now := s.now() // 获取当前时间
	// jwks_keys.not_before is DATETIME(0). MySQL rounds fractional seconds when
	// storing it, so an immediate key created late in a second could otherwise
	// read back with a not-before time in the future and fail startup validation.
	effectiveNotBefore := now.Truncate(time.Second)
	if notBefore != nil {
		effectiveNotBefore = *notBefore // 设置生效时间
	}
	// 如果生效时间大于当前时间，则返回错误
	if effectiveNotBefore.After(now) {
		return keyActivationResult{}, errors.WithCode(code.ErrInvalidTimeRange, "NotBefore cannot be in the future for an active key")
	}
	// 计算过期时间
	effectiveNotAfter := now.Add(s.policy.RotationInterval + s.policy.GracePeriod)
	// 如果过期时间小于生效时间，则返回错误
	if notAfter != nil {
		effectiveNotAfter = *notAfter // 设置过期时间
	}
	// 如果过期时间小于生效时间，则返回错误
	if !effectiveNotAfter.After(effectiveNotBefore) {
		return keyActivationResult{}, errors.WithCode(code.ErrInvalidTimeRange, "NotAfter must be after NotBefore")
	}

	// 生成密钥ID
	kid := "key-" + uuid.NewString()
	// 生成密钥对
	keyPair, err := s.keyGenerator.GenerateKeyPair(ctx, alg, kid)
	if err != nil {
		return keyActivationResult{}, errors.WithCode(code.ErrUnknown, "failed to generate key pair: %v", err)
	}
	// 创建候选密钥
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

	// 创建激活请求
	request := ActivationRequest{
		Candidate:  candidate,
		Now:        now,
		GraceUntil: now.Add(s.policy.GracePeriod),
	}
	// 根据激活模式设置激活请求
	switch mode {
	case activationModeBootstrap:
		request.RequireNoActive = true
	case activationModeIfDue:
		dueBefore := now.Add(-s.policy.RotationInterval)
		request.DueBefore = &dueBefore
	}
	// 激活密钥
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
	// 返回激活结果
	return keyActivationResult{Active: activation.Active, Activated: activation.Activated}, nil
}

// deleteCandidatePEM 删除候选密钥
func (s *KeyManager) deleteCandidatePEM(ctx context.Context, kid string) {
	// 如果私钥存储未配置，则返回
	if s.privateStore == nil {
		return
	}
	// 删除候选密钥
	if err := s.privateStore.DeletePrivateKey(ctx, kid); err != nil && !errors.IsCode(err, code.ErrKeyNotFound) {
		candidateCleanupFailures.Inc()
		log.Warnw("failed to remove unused jwks candidate", "kid", kid)
	}
}

// GetActiveKey 获取当前激活的密钥
func (s *KeyManager) GetActiveKey(ctx context.Context) (*Key, error) {
	// 查询当前激活的密钥
	keys, err := s.keyRepo.FindByStatus(ctx, KeyActive)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find active keys: %v", err)
	}

	// 如果当前没有激活的密钥，则返回错误
	if len(keys) == 0 {
		return nil, errors.WithCode(code.ErrNoActiveKey, "no active key available")
	}
	// 如果当前有多个激活的密钥，则返回错误
	if len(keys) != 1 {
		return nil, errors.WithCode(code.ErrInvalidStateTransition, "expected exactly one active key, found %d", len(keys))
	}

	// 过滤出可以用于签名的密钥（未过期且状态正确）
	now := s.now()
	for _, key := range keys {
		if key.CanSignAt(now) {
			return key, nil
		}
	}

	// 如果当前没有有效的激活密钥，则返回错误
	return nil, errors.WithCode(code.ErrNoActiveKey, "no valid active key available")
}

// ValidateActiveKey 验证当前激活的密钥是否有效，且其 PEM 私钥与 MySQL 中存储的公 JWK 匹配。
func (s *KeyManager) ValidateActiveKey(ctx context.Context, resolver PrivateKeyResolver) (*Key, error) {
	// 获取当前激活的密钥
	active, err := s.GetActiveKey(ctx)
	// 如果获取失败，则返回错误
	if err != nil {
		return nil, err
	}
	// 如果私钥解析器未配置，则返回错误
	if resolver == nil {
		return nil, errors.WithCode(code.ErrUnknown, "private key resolver is not configured")
	}
	// 解析私钥
	privateKey, err := resolver.ResolveSigningKey(ctx, active.Kid, active.Algorithm)
	if err != nil {
		materialValidationFailures.WithLabelValues("resolve").Inc()
		return nil, errors.WithCode(code.ErrUnknown, "resolve active private key %s: %v", active.Kid, err)
	}
	// 如果私钥类型不匹配，则返回错误
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		materialValidationFailures.WithLabelValues("type").Inc()
		return nil, errors.WithCode(code.ErrInvalidJWK, "active private key %s is not RSA", active.Kid)
	}
	// 计算公钥的 N 和 E
	expectedN := base64.RawURLEncoding.EncodeToString(rsaKey.PublicKey.N.Bytes())
	expectedE := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.PublicKey.E)).Bytes())
	// 如果公钥的 N 和 E 不匹配，则返回错误
	if active.JWK.N == nil || active.JWK.E == nil || *active.JWK.N != expectedN || *active.JWK.E != expectedE {
		materialValidationFailures.WithLabelValues("mismatch").Inc()
		return nil, errors.WithCode(code.ErrInvalidJWK, "active private key does not match public JWK for kid %s", active.Kid)
	}
	// 返回激活的密钥
	return active, nil
}

// GetKeyByKid 根据 kid 获取密钥
func (s *KeyManager) GetKeyByKid(ctx context.Context, kid string) (*Key, error) {
	// 根据 kid 获取密钥
	key, err := s.keyRepo.FindByKid(ctx, kid)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find key: %v", err)
	}

	// 如果密钥不存在，则返回错误
	if key == nil {
		return nil, errors.WithCode(code.ErrKeyNotFound, "key not found: %s", kid)
	}

	return key, nil
}

// RetireKey 退役密钥（Grace → Retired）
func (s *KeyManager) RetireKey(ctx context.Context, kid string) error {
	// 根据 kid 获取密钥
	key, err := s.keyRepo.FindByKid(ctx, kid)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to find key: %v", err)
	}

	// 如果密钥不存在，则返回错误
	if key == nil {
		return errors.WithCode(code.ErrKeyNotFound, "key not found: %s", kid)
	}
	// 退役密钥（Grace → Retired）
	if err := key.Retire(s.now()); err != nil {
		return err
	}

	// 更新密钥状态
	if err := s.keyRepo.Update(ctx, key); err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to update key: %v", err)
	}

	return nil
}

// ForceRetireKey 强制退役密钥（任何状态 → Retired）
func (s *KeyManager) ForceRetireKey(ctx context.Context, kid string) error {
	// 根据 kid 获取密钥
	key, err := s.keyRepo.FindByKid(ctx, kid)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to find key: %v", err)
	}

	if key == nil {
		return errors.WithCode(code.ErrKeyNotFound, "key not found: %s", kid)
	}
	// 强制状态转换（任何状态 → Retired）
	if err := key.ForceRetire(s.now()); err != nil {
		return err
	}

	// 保存状态
	if err := s.keyRepo.Update(ctx, key); err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to update key: %v", err)
	}

	return nil
}

// CleanupExpiredKeys 清理过期密钥
// 删除 NotAfter < now 且 Status = Retired 的密钥，并删除私钥。
func (s *KeyManager) CleanupExpiredKeys(ctx context.Context) (int, error) {
	// 查询已过期的密钥
	expiredKeys, err := s.keyRepo.FindExpired(ctx)
	if err != nil {
		return 0, errors.WithCode(code.ErrDatabase, "failed to find expired keys: %v", err)
	}

	// 如果已过期的密钥为空，则返回
	if len(expiredKeys) == 0 {
		return 0, nil
	}

	// 过期的非活动密钥不再可发布。首先退役并删除数据库行，然后删除私钥。
	// database row first, then remove the private material.
	deletedCount := 0
	for _, key := range expiredKeys {
		// 如果密钥是活动的，则跳过
		if key.IsActive() {
			continue
		}
		// 如果密钥不是退役的，则强制退役
		if !key.IsRetired() {
			if err := key.ForceRetire(s.now()); err != nil {
				continue
			}
			// 更新密钥状态
			if err := s.keyRepo.Update(ctx, key); err != nil {
				continue
			}
		}
		// 删除密钥
		if err := s.keyRepo.Delete(ctx, key.Kid); err != nil {
			continue
		}
		deletedCount++
		// 如果私钥存储未配置，则跳过
		if s.privateStore != nil {
			if err := s.privateStore.DeletePrivateKey(ctx, key.Kid); err != nil && !errors.IsCode(err, code.ErrKeyNotFound) {
				// 记录删除失败
				recordPostCommitFailure("private_key_delete")
				// 记录删除失败日志
				log.Warnw("failed to delete retired jwks private key", "kid", key.Kid)
			}
		}
	}

	// 返回删除的密钥数量
	return deletedCount, nil
}

// ListKeys 列出密钥（分页）
func (s *KeyManager) ListKeys(
	ctx context.Context,
	status KeyStatus,
	limit, offset int,
) ([]*Key, int64, error) {
	// 如果指定了状态，则按状态查询
	if status != 0 {
		keys, err := s.keyRepo.FindByStatus(ctx, status)
		if err != nil {
			return nil, 0, errors.WithCode(code.ErrDatabase, "failed to find keys: %v", err)
		}

		// 手动分页，计算总数
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
