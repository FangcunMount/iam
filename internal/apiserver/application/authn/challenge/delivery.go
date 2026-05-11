package challenge

import (
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// SMSOTPDelivery 短信验证码发送依赖
type SMSOTPDelivery struct {
	Gate     authentication.OTPSendGate
	SMS      authentication.SMSSender
	TTL      time.Duration
	Cooldown time.Duration
	CodeLen  int
}

// effectiveTTL 获取有效的 TTL
func (d *SMSOTPDelivery) effectiveTTL() time.Duration {
	if d == nil || d.TTL <= 0 {
		return defaultSMSOTPTTL
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
		return defaultSMSOTPCodeLen
	}
	if d.CodeLen > 12 {
		return 12
	}
	return d.CodeLen
}
