package redis

import (
	"context"
	"fmt"
	"time"

	redisops "github.com/FangcunMount/component-base/pkg/redis/ops"
	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	"github.com/redis/go-redis/v9"

	"github.com/FangcunMount/component-base/pkg/log"
	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v2/internal/apiserver/cache"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// OTPVerifierImpl OTP验证器的Redis实现
type OTPVerifierImpl struct {
	client    *redis.Client
	otpCodes  *redisstore.ValueStore[string]
	sendGates *redisstore.ValueStore[string]
}

// 确保实现了接口
var (
	_ authentication.OTPCodeStore = (*OTPVerifierImpl)(nil)
	_ authentication.OTPSendGate  = (*OTPVerifierImpl)(nil)
	_ authentication.OTPSendQuota = (*OTPVerifierImpl)(nil)
)

// NewOTPVerifier 创建 OTP Redis 适配器（验证、写入、发送频控共用同一实现）
func NewOTPVerifier(client *redis.Client) *OTPVerifierImpl {
	return &OTPVerifierImpl{
		client:    client,
		otpCodes:  newStringStore(client),
		sendGates: newStringStore(client),
	}
}

// FamilyInspectors 返回 OTP 相关缓存族的状态读取器。
func (v *OTPVerifierImpl) FamilyInspectors() []cachegovernance.FamilyInspector {
	return []cachegovernance.FamilyInspector{
		newRedisFamilyInspector(cachemodel.FamilyAuthnLoginOTPSendGate, v.client, "发送频控采用 SET NX EX 的 cooldown marker。"),
	}
}

// VerifyAndConsume 验证OTP并标记为已使用（原子操作，防止重放攻击）
// phoneE164: E164格式的手机号，如 +8613800138000
// scene: OTP使用场景，如 "login", "register", "reset_password"
// code: 验证码
func (v *OTPVerifierImpl) VerifyAndConsume(ctx context.Context, phoneE164, scene, code string) bool {
	key := otpRedisKey(phoneE164, scene, code)

	result, err := redisops.ConsumeIfExists(ctx, v.client, key)
	if err != nil {
		return false
	}

	if result {
		redisInfo(ctx, "OTP verified",
			log.String("scene", scene),
			log.String("phone", phoneE164),
		)
	}

	return result
}

// Put 写入待校验 OTP，与 VerifyAndConsume 使用同一 key 规则。
func (v *OTPVerifierImpl) Put(ctx context.Context, phoneE164, scene, code string, ttl time.Duration) error {
	key := otpRedisKey(phoneE164, scene, code)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return err
	}
	return v.otpCodes.Set(ctx, storeKey, "1", ttl)
}

// Delete 删除 OTP 键（短信发送失败时回滚）。
func (v *OTPVerifierImpl) Delete(ctx context.Context, phoneE164, scene, code string) error {
	key := otpRedisKey(phoneE164, scene, code)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return err
	}
	return v.otpCodes.Delete(ctx, storeKey)
}

// TryAcquire 使用 SET NX 实现发送冷却窗口。
func (v *OTPVerifierImpl) TryAcquire(ctx context.Context, phoneE164, scene string, cooldown time.Duration) (bool, error) {
	key := otpSendGateRedisKey(phoneE164, scene)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return false, fmt.Errorf("otp send gate: %w", err)
	}
	ok, err := v.sendGates.SetIfAbsent(ctx, storeKey, "1", cooldown)
	if err != nil {
		return false, fmt.Errorf("otp send gate: %w", err)
	}
	return ok, nil
}

// TryConsume 固定窗口计数：INCR 当前窗口计数器，首次写入时设置过期时间。
func (v *OTPVerifierImpl) TryConsume(ctx context.Context, phoneE164, scene, dimension string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return true, nil
	}
	key := otpSendQuotaRedisKey(phoneE164, scene, dimension, otpQuotaBucket(window))
	n, err := v.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("otp send quota incr: %w", err)
	}
	if n == 1 {
		if err := v.client.Expire(ctx, key, window).Err(); err != nil {
			return false, fmt.Errorf("otp send quota expire: %w", err)
		}
	}
	if n > int64(limit) {
		// 超限拒绝时回退本次 INCR，避免“探测性请求”占用配额。
		if _, err := v.client.Decr(ctx, key).Result(); err != nil {
			return false, fmt.Errorf("otp send quota rollback incr: %w", err)
		}
		return false, nil
	}
	return true, nil
}

// Rollback 回退一次计数（发送失败时调用）；DECR 不应使计数低于 0。
func (v *OTPVerifierImpl) Rollback(ctx context.Context, phoneE164, scene, dimension string, window time.Duration) error {
	if window <= 0 {
		return nil
	}
	key := otpSendQuotaRedisKey(phoneE164, scene, dimension, otpQuotaBucket(window))
	n, err := v.client.Decr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("otp send quota decr: %w", err)
	}
	if n < 0 {
		// 计数器异常（窗口已过期重建），归零并保留 TTL 由后续写入重置。
		if err := v.client.Set(ctx, key, 0, window).Err(); err != nil {
			return fmt.Errorf("otp send quota reset: %w", err)
		}
	}
	return nil
}

// otpQuotaBucket 将当前时间按窗口长度对齐，作为固定窗口的桶标识。
func otpQuotaBucket(window time.Duration) string {
	return fmt.Sprintf("%d", time.Now().Truncate(window).Unix())
}
