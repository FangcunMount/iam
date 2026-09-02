package ratelimit

import (
	"math"

	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
)

// Config REST 层按操作员限流；PerOperatorQPS<=0 表示关闭。
type Config struct {
	PerOperatorQPS                float64
	PerOperatorBurst              int
	MobileKeywordPerOperatorQPS   float64
	MobileKeywordPerOperatorBurst int
	Backend                       string
	OperatorMapMaxEntries         int
}

// ConfigFromOptions 从 SuggestOptions 映射限流配置。
func ConfigFromOptions(o apiserveroptions.SuggestOptions) Config {
	cfg := Config{
		PerOperatorQPS:                o.RateLimit.PerOperatorQPS,
		PerOperatorBurst:              o.RateLimit.PerOperatorBurst,
		MobileKeywordPerOperatorQPS:   o.RateLimit.MobileKeywordPerOperatorQPS,
		MobileKeywordPerOperatorBurst: o.RateLimit.MobileKeywordPerOperatorBurst,
		Backend:                       o.RateLimit.Backend,
		OperatorMapMaxEntries:         o.RateLimit.OperatorMapMaxEntries,
	}
	return cfg.WithDefaults()
}

// WithDefaults 填充默认值。
func (r Config) WithDefaults() Config {
	if r.PerOperatorQPS <= 0 {
		return Config{}
	}
	out := r
	if out.PerOperatorBurst <= 0 {
		out.PerOperatorBurst = max(5, int(math.Ceil(out.PerOperatorQPS*2)))
	}
	if out.MobileKeywordPerOperatorQPS <= 0 {
		out.MobileKeywordPerOperatorQPS = out.PerOperatorQPS
	}
	if out.MobileKeywordPerOperatorBurst <= 0 {
		out.MobileKeywordPerOperatorBurst = max(3, int(math.Ceil(out.MobileKeywordPerOperatorQPS*2)))
	}
	if out.OperatorMapMaxEntries <= 0 {
		out.OperatorMapMaxEntries = 10000
	}
	if out.Backend == "" {
		out.Backend = "memory"
	}
	return out
}
