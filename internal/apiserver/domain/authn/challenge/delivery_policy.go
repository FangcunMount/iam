package challenge

import "time"

// SMSOTPDeliveryConfig 是发送策略解析的原始配置输入。
type SMSOTPDeliveryConfig struct {
	TTL         time.Duration // 过期时间
	Cooldown    time.Duration // 冷却时间
	CodeLen     int           // 验证码长度
	HourlyLimit int           // 小时限制
	DailyLimit  int           // 每日限制
}

// SMSOTPDeliveryPolicy 描述短信 OTP 发送治理策略。
type SMSOTPDeliveryPolicy struct {
	TTL         time.Duration // 过期时间
	Cooldown    time.Duration // 冷却时间
	CodeLen     int           // 验证码长度
	HourlyLimit int           // 小时限制
	DailyLimit  int           // 每日限制
}

const (
	// 短信验证码默认发送策略
	DefaultSMSOTPSendCooldown = 60 * time.Second // 60秒冷却时间
	DefaultSMSOTPHourlyLimit  = 5                // 5次小时限制
	DefaultSMSOTPDailyLimit   = 10               // 10次每日限制

	// 配额维度
	QuotaDimensionHourly = "hourly"       // 小时维度
	QuotaDimensionDaily  = "daily"        // 每日维度
	QuotaWindowHourly    = time.Hour      // 1小时窗口
	QuotaWindowDaily     = 24 * time.Hour // 24小时窗口
)

// ResolveSMSOTPDeliveryPolicy 将配置解析为有效发送策略。
func ResolveSMSOTPDeliveryPolicy(cfg SMSOTPDeliveryConfig) SMSOTPDeliveryPolicy {
	// 解析配置
	policy := SMSOTPDeliveryPolicy{
		TTL:         cfg.TTL,
		Cooldown:    cfg.Cooldown,
		CodeLen:     cfg.CodeLen,
		HourlyLimit: cfg.HourlyLimit,
		DailyLimit:  cfg.DailyLimit,
	}

	// 检验过期时间
	if policy.TTL <= 0 {
		policy.TTL = DefaultSMSOTPTTL
	}

	// 检验冷却时间
	if policy.Cooldown <= 0 {
		policy.Cooldown = DefaultSMSOTPSendCooldown
	}

	// 检验验证码长度
	if policy.CodeLen <= 0 {
		policy.CodeLen = DefaultSMSOTPCodeLen
	} else if policy.CodeLen > MaxSMSOTPCodeLen {
		policy.CodeLen = MaxSMSOTPCodeLen
	}

	// 返回有效发送策略
	return policy
}

// EffectiveHourlyLimit 返回有效的小时配额（0 表示关闭）。
func (p SMSOTPDeliveryPolicy) EffectiveHourlyLimit() int {
	// 如果小时限制为0，则返回默认小时限制
	if p.HourlyLimit == 0 {
		return DefaultSMSOTPHourlyLimit
	}

	// 如果小时限制小于0，则返回0
	if p.HourlyLimit < 0 {
		return 0
	}

	// 返回有效小时限制
	return p.HourlyLimit
}

// EffectiveDailyLimit 返回有效的每日配额（0 表示关闭）。
func (p SMSOTPDeliveryPolicy) EffectiveDailyLimit() int {
	// 如果每日限制为0，则返回默认每日限制
	if p.DailyLimit == 0 {
		return DefaultSMSOTPDailyLimit
	}

	// 如果每日限制小于0，则返回0
	if p.DailyLimit < 0 {
		return 0
	}

	// 返回有效每日限制
	return p.DailyLimit
}

// QuotaDimensions 返回按顺序消费的发送配额维度。
func QuotaDimensions() []struct {
	Dimension string
	Window    time.Duration
} {
	return []struct {
		Dimension string
		Window    time.Duration
	}{
		{QuotaDimensionHourly, QuotaWindowHourly},
		{QuotaDimensionDaily, QuotaWindowDaily},
	}
}
