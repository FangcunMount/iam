package challenge

import (
	"time"

	challengeDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
)

const (
	defaultSMSOTPHourlyLimit = 5  // 默认每小时发送上限
	defaultSMSOTPDailyLimit  = 10 // 默认每天发送上限
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

// effectiveTTL 获取有效的 TTL
func (d *SMSOTPDelivery) effectiveTTL() time.Duration {
	if d == nil || d.TTL <= 0 {
		return challengeDomain.DefaultSMSOTPTTL
	}
	return d.TTL
}

// effectiveCooldown 获取有效的冷却时间
func (d *SMSOTPDelivery) effectiveCooldown() time.Duration {
	if d == nil || d.Cooldown <= 0 {
		return 60 * time.Second
	}
	return d.Cooldown
}

// effectiveCodeLen 获取有效的验证码长度
func (d *SMSOTPDelivery) effectiveCodeLen() int {
	if d == nil || d.CodeLen <= 0 {
		return challengeDomain.DefaultSMSOTPCodeLen
	}
	if d.CodeLen > challengeDomain.MaxSMSOTPCodeLen {
		return challengeDomain.MaxSMSOTPCodeLen
	}
	return d.CodeLen
}

// effectiveHourlyLimit 获取有效的每小时发送上限（<0 视为关闭，0 取默认）。
func (d *SMSOTPDelivery) effectiveHourlyLimit() int {
	if d == nil || d.HourlyLimit == 0 {
		return defaultSMSOTPHourlyLimit
	}
	if d.HourlyLimit < 0 {
		return 0
	}
	return d.HourlyLimit
}

// effectiveDailyLimit 获取有效的每天发送上限（<0 视为关闭，0 取默认）。
func (d *SMSOTPDelivery) effectiveDailyLimit() int {
	if d == nil || d.DailyLimit == 0 {
		return defaultSMSOTPDailyLimit
	}
	if d.DailyLimit < 0 {
		return 0
	}
	return d.DailyLimit
}
