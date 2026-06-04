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
	now       func() time.Time
}

// 确保实现了接口
var (
	_ authentication.OTPCodeStore = (*OTPVerifierImpl)(nil)
	_ authentication.OTPSendGate  = (*OTPVerifierImpl)(nil)
	_ authentication.OTPSendQuota = (*OTPVerifierImpl)(nil)
)

// NewOTPVerifier 创建 OTP Redis 适配器（验证、写入、发送频控共用同一实现）
func NewOTPVerifier(client *redis.Client) *OTPVerifierImpl {
	return newOTPVerifierWithClock(client, time.Now)
}

func newOTPVerifierWithClock(client *redis.Client, now func() time.Time) *OTPVerifierImpl {
	if now == nil {
		now = time.Now
	}
	return &OTPVerifierImpl{
		client:    client,
		otpCodes:  newStringStore(client),
		sendGates: newStringStore(client),
		now:       now,
	}
}

// FamilyInspectors 返回 OTP 相关缓存族的状态读取器。
func (v *OTPVerifierImpl) FamilyInspectors() []cachegovernance.FamilyInspector {
	return []cachegovernance.FamilyInspector{
		newRedisFamilyInspector(cachemodel.FamilyAuthnLoginOTPSendGate, v.client, "发送频控采用 SET NX EX 的 cooldown marker。"),
		newRedisFamilyInspector(cachemodel.FamilyAuthnLoginOTPSendQuota, v.client, "发送限量采用 Redis Lua 维护固定窗口计数器。"),
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

const otpQuotaTryConsumeLua = `
local n = redis.call("INCR", KEYS[1])
local ttl = redis.call("PTTL", KEYS[1])
if n == 1 or ttl < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
if n > tonumber(ARGV[2]) then
  local rolled = redis.call("DECR", KEYS[1])
  if rolled <= 0 then
    redis.call("DEL", KEYS[1])
  end
  return 0
end
return 1
`

const otpQuotaRollbackLua = `
local current = redis.call("GET", KEYS[1])
if not current then
  return 0
end
local n = tonumber(current)
if not n or n <= 1 then
  redis.call("DEL", KEYS[1])
  return 0
end
redis.call("DECR", KEYS[1])
return 1
`

// TryConsume 固定窗口计数：原子累计当前窗口计数器并确保 TTL 存在。
func (v *OTPVerifierImpl) TryConsume(ctx context.Context, phoneE164, scene, dimension string, limit int, window time.Duration) (authentication.OTPSendQuotaLease, bool, error) {
	if limit <= 0 || window <= 0 {
		return authentication.OTPSendQuotaLease{}, true, nil
	}
	bucket := otpQuotaBucketAt(v.now(), window)
	key := otpSendQuotaRedisKey(phoneE164, scene, dimension, bucket)
	ttlMillis := int64(window / time.Millisecond)
	if ttlMillis <= 0 {
		ttlMillis = 1
	}
	res, err := v.client.Eval(ctx, otpQuotaTryConsumeLua, []string{key}, ttlMillis, limit).Int()
	if err != nil {
		return authentication.OTPSendQuotaLease{}, false, fmt.Errorf("otp send quota consume: %w", err)
	}
	if res == 0 {
		return authentication.OTPSendQuotaLease{}, false, nil
	}
	return authentication.OTPSendQuotaLease{
		PhoneE164: phoneE164,
		Scene:     scene,
		Dimension: dimension,
		Bucket:    bucket,
		Window:    window,
	}, true, nil
}

// Rollback 回退 lease 对应的一次计数（发送失败时调用）；不会产生负数或长期 0 key。
func (v *OTPVerifierImpl) Rollback(ctx context.Context, lease authentication.OTPSendQuotaLease) error {
	if lease.IsZero() {
		return nil
	}
	key := otpSendQuotaRedisKey(lease.PhoneE164, lease.Scene, lease.Dimension, lease.Bucket)
	return v.client.Eval(ctx, otpQuotaRollbackLua, []string{key}).Err()
}

// otpQuotaBucket 将当前时间按窗口长度对齐，作为固定窗口的桶标识。
func otpQuotaBucket(window time.Duration) string {
	return otpQuotaBucketAt(time.Now(), window)
}

func otpQuotaBucketAt(now time.Time, window time.Duration) string {
	return fmt.Sprintf("%d", now.Truncate(window).Unix())
}
