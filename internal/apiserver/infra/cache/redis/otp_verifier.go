package redis

import (
	"context"
	"fmt"
	"time"

	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v2/internal/apiserver/cache"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// OTPVerifierImpl OTP验证器的Redis实现
type OTPVerifierImpl struct {
	client    *redis.Client
	sendGates *redisstore.ValueStore[string]
	now       func() time.Time
}

// 确保实现了接口
var (
	_ authentication.OTPSendGate  = (*OTPVerifierImpl)(nil)
	_ authentication.OTPSendQuota = (*OTPVerifierImpl)(nil)
)

// NewOTPVerifier 创建 OTP 发送频控 Redis 适配器。
func NewOTPVerifier(client *redis.Client) *OTPVerifierImpl {
	return newOTPVerifierWithClock(client, time.Now)
}

func newOTPVerifierWithClock(client *redis.Client, now func() time.Time) *OTPVerifierImpl {
	if now == nil {
		now = time.Now
	}
	return &OTPVerifierImpl{
		client:    client,
		sendGates: newStringStore(client),
		now:       now,
	}
}

// FamilyInspectors 返回 OTP 相关缓存族的状态读取器。
func (v *OTPVerifierImpl) FamilyInspectors() []cachegovernance.FamilyInspector {
	return []cachegovernance.FamilyInspector{
		newRedisFamilyInspector(cachemodel.FamilyAuthnLoginOTPSendGate, v.client, "发送频控采用 SET NX EX 的 cooldown marker。"),
		newRedisFamilyInspector(cachemodel.FamilyAuthnLoginOTPSendQuota, v.client, "发送限量采用 Redis Lua 维护滑动窗口 ZSET。"),
	}
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

// otpQuotaTryConsumeLua 滑动窗口计数：清理窗口外记录后，原子追加本次发送记录并设置 TTL。
const otpQuotaTryConsumeLua = `
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
local cutoff = now - window
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", cutoff)
local count = redis.call("ZCARD", KEYS[1])
if count >= limit then
  local ttl = redis.call("PTTL", KEYS[1])
  if ttl < 0 then
    redis.call("PEXPIRE", KEYS[1], window)
  end
  return 0
end
redis.call("ZADD", KEYS[1], now, member)
redis.call("PEXPIRE", KEYS[1], window)
return 1
`

// otpQuotaRollbackLua 回退 lease 对应的一次滑动窗口成员（发送失败时调用）；不会产生负数或长期空 key。
const otpQuotaRollbackLua = `
redis.call("ZREM", KEYS[1], ARGV[1])
local count = redis.call("ZCARD", KEYS[1])
if count <= 0 then
  redis.call("DEL", KEYS[1])
  return 0
end
redis.call("PEXPIRE", KEYS[1], ARGV[2])
return 1
`

// TryConsume 滑动窗口计数：清理窗口外记录后，原子追加本次发送记录并设置 TTL。
func (v *OTPVerifierImpl) TryConsume(ctx context.Context, phoneE164, scene, dimension string, limit int, window time.Duration) (authentication.OTPSendQuotaLease, bool, error) {
	if limit <= 0 || window <= 0 {
		return authentication.OTPSendQuotaLease{}, true, nil
	}
	nowMillis := v.now().UnixMilli()
	windowMillis := int64(window / time.Millisecond)
	if windowMillis <= 0 {
		windowMillis = 1
	}
	member := otpQuotaMember(nowMillis)
	key := otpSendQuotaRedisKey(phoneE164, scene, dimension)
	res, err := v.client.Eval(ctx, otpQuotaTryConsumeLua, []string{key}, nowMillis, windowMillis, limit, member).Int()
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
		Member:    member,
		Window:    window,
	}, true, nil
}

// Rollback 回退 lease 对应的一次滑动窗口成员（发送失败时调用）；不会产生负数或长期空 key。
func (v *OTPVerifierImpl) Rollback(ctx context.Context, lease authentication.OTPSendQuotaLease) error {
	if lease.IsZero() {
		return nil
	}
	windowMillis := int64(lease.Window / time.Millisecond)
	if windowMillis <= 0 {
		windowMillis = 1
	}
	key := otpSendQuotaRedisKey(lease.PhoneE164, lease.Scene, lease.Dimension)
	return v.client.Eval(ctx, otpQuotaRollbackLua, []string{key}, lease.Member, windowMillis).Err()
}

// otpQuotaMember 构造滑动窗口成员的唯一标识符（时间戳 + 随机数）。
func otpQuotaMember(nowMillis int64) string {
	return fmt.Sprintf("%d:%s", nowMillis, uuid.NewString())
}
