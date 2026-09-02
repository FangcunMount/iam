package challenge

import (
	"context"
	"time"
)

// OTPSendGate 限制同一手机号、同一场景的发送频率。
type OTPSendGate interface {
	TryAcquire(ctx context.Context, phoneE164, scene string, cooldown time.Duration) (bool, error)
}

// OTPSendQuotaLease 表示一次配额消费，用于精确回滚。
type OTPSendQuotaLease struct {
	PhoneE164 string
	Scene     string
	Dimension string
	Member    string
	Window    time.Duration
}

func (l OTPSendQuotaLease) IsZero() bool {
	return l.PhoneE164 == "" || l.Scene == "" || l.Dimension == "" || l.Member == "" || l.Window <= 0
}

// OTPSendQuota 限制同一手机号、同一场景在滑动时间窗口内的累计发送次数。
type OTPSendQuota interface {
	TryConsume(ctx context.Context, phoneE164, scene, dimension string, limit int, window time.Duration) (OTPSendQuotaLease, bool, error)
	Rollback(ctx context.Context, lease OTPSendQuotaLease) error
}

// SMSSender 登录 OTP 触达通道。
type SMSSender interface {
	SendLoginOTP(ctx context.Context, phoneE164, code string) error
}
