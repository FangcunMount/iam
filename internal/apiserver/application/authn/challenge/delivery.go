package challenge

import (
	"time"

	challengeDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
)

// SMSOTPDelivery 短信验证码发送依赖
type SMSOTPDelivery struct {
	Gate        OTPSendGate
	Quota       OTPSendQuota
	SMS         SMSSender
	TTL         time.Duration
	Cooldown    time.Duration
	CodeLen     int
	HourlyLimit int
	DailyLimit  int
}

func (d *SMSOTPDelivery) policy() challengeDomain.SMSOTPDeliveryPolicy {
	if d == nil {
		return challengeDomain.ResolveSMSOTPDeliveryPolicy(challengeDomain.SMSOTPDeliveryConfig{})
	}
	return challengeDomain.ResolveSMSOTPDeliveryPolicy(challengeDomain.SMSOTPDeliveryConfig{
		TTL:         d.TTL,
		Cooldown:    d.Cooldown,
		CodeLen:     d.CodeLen,
		HourlyLimit: d.HourlyLimit,
		DailyLimit:  d.DailyLimit,
	})
}

// effectiveTTL 获取有效的 TTL
func (d *SMSOTPDelivery) effectiveTTL() time.Duration {
	return d.policy().TTL
}

// effectiveCooldown 获取有效的冷却时间
func (d *SMSOTPDelivery) effectiveCooldown() time.Duration {
	return d.policy().Cooldown
}

// effectiveCodeLen 获取有效的验证码长度
func (d *SMSOTPDelivery) effectiveCodeLen() int {
	return d.policy().CodeLen
}

// effectiveHourlyLimit 获取有效的每小时发送上限（<0 视为关闭，0 取默认）。
func (d *SMSOTPDelivery) effectiveHourlyLimit() int {
	return d.policy().EffectiveHourlyLimit()
}

// effectiveDailyLimit 获取有效的每天发送上限（<0 视为关闭，0 取默认）。
func (d *SMSOTPDelivery) effectiveDailyLimit() int {
	return d.policy().EffectiveDailyLimit()
}
