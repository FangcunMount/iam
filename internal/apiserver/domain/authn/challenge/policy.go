package challenge

import "time"

const (
	DefaultSMSOTPTTL         = 5 * time.Minute // 短信验证码默认过期时间
	DefaultSMSOTPCodeLen     = 6               // 短信验证码默认长度
	MaxSMSOTPCodeLen         = 12              // 短信验证码最大长度
	DefaultMaxVerifyAttempts = 5               // 默认最大验证尝试次数

	DefaultOAuthStateTTL = 10 * time.Minute // OAuth状态默认过期时间

)
