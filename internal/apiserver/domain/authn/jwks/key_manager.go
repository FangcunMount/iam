package jwks

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/google/uuid"
)

// KeyManager 密钥生命周期管理服务
// 实现 Manager 接口
type KeyManager struct {
	keyRepo      Repository
	keyGenerator KeyGenerator
}

// NewKeyManager 创建密钥管理器
func NewKeyManager(
	keyRepo Repository,
	keyGenerator KeyGenerator,
) *KeyManager {
	return &KeyManager{
		keyRepo:      keyRepo,
		keyGenerator: keyGenerator,
	}
}

// Ensure KeyManager implements KeyManagementService
var _ Manager = (*KeyManager)(nil)

// CreateKey 创建新密钥
func (s *KeyManager) CreateKey(
	ctx context.Context,
	alg string,
	notBefore, notAfter *time.Time,
) (*Key, error) {
	// 生成 kid (UUID)
	kid := uuid.New().String()

	// 使用密钥生成器生成密钥对
	keyPair, err := s.keyGenerator.GenerateKeyPair(ctx, alg, kid)
	if err != nil {
		return nil, errors.WithCode(code.ErrUnknown, "failed to generate key pair: %v", err)
	}

	// 构建 KeyOption
	var opts []KeyOption
	if notBefore != nil {
		opts = append(opts, WithNotBefore(*notBefore))
	} else {
		// 默认立即生效
		now := time.Now()
		opts = append(opts, WithNotBefore(now))
	}

	if notAfter != nil {
		opts = append(opts, WithNotAfter(*notAfter))
	}

	// 默认状态为 Active
	opts = append(opts, WithStatus(KeyActive))

	// 创建密钥实体
	key := NewKey(kid, keyPair.PublicJWK, opts...)

	// 验证密钥
	if err := key.Validate(); err != nil {
		return nil, err
	}

	// 保存密钥
	if err := s.keyRepo.Save(ctx, key); err != nil {
		// 如果是重复键错误（可能由并发创建导致），尝试读取已存在的密钥并返回它，
		// 这样启动时的竞态不会导致致命错误。
		if errors.IsCode(err, code.ErrKeyAlreadyExists) {
			// 直接从仓储查询已有的 key（按 kid）
			existing, findErr := s.keyRepo.FindByKid(ctx, kid)
			if findErr != nil {
				return nil, errors.WithCode(code.ErrDatabase, "failed to save key and failed to fetch existing key: %v; fetch err: %v", err, findErr)
			}
			if existing != nil {
				return existing, nil
			}
			// 如果未找到（极不可能），回退为通用数据库错误
			return nil, errors.WithCode(code.ErrDatabase, "failed to save key: %v", err)
		}

		return nil, errors.WithCode(code.ErrDatabase, "failed to save key: %v", err)
	}

	return key, nil
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

	// 过滤出可以用于签名的密钥（未过期且状态正确）
	now := time.Now()
	for _, key := range keys {
		if key.CanSign() && key.IsValidAt(now) {
			return key, nil
		}
	}

	return nil, errors.WithCode(code.ErrNoActiveKey, "no valid active key available")
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

	// 只删除 Retired 状态的过期密钥
	deletedCount := 0
	for _, key := range expiredKeys {
		if key.Status == KeyRetired {
			if err := s.keyRepo.Delete(ctx, key.Kid); err != nil {
				// 继续删除其他密钥，记录错误
				continue
			}
			deletedCount++
		} else {
			// 如果过期但不是 Retired 状态，强制退役
			key.ForceRetire()
			if err := s.keyRepo.Update(ctx, key); err != nil {
				// 继续处理其他密钥
				continue
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
