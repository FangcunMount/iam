package suggest

import (
	"sync"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	"golang.org/x/time/rate"
)

// PerOperatorRateLimiter 进程内按 operator 分桶限流；nil 表示关闭。
type PerOperatorRateLimiter struct {
	mu sync.Mutex

	std map[int64]*rate.Limiter
	mob map[int64]*rate.Limiter

	stdRate  rate.Limit
	stdBurst int
	mobRate  rate.Limit
	mobBurst int
}

// NewPerOperatorRateLimiterFromConfig 根据模块配置构造限流器；配置关闭时返回 nil。
func NewPerOperatorRateLimiterFromConfig(cfg appsuggest.RateLimitConfig) *PerOperatorRateLimiter {
	if cfg.PerOperatorQPS <= 0 {
		return nil
	}
	stdBurst := cfg.PerOperatorBurst
	if stdBurst <= 0 {
		stdBurst = 5
	}
	mobRate := cfg.MobileKeywordPerOperatorQPS
	if mobRate <= 0 {
		mobRate = cfg.PerOperatorQPS
	}
	mobBurst := cfg.MobileKeywordPerOperatorBurst
	if mobBurst <= 0 {
		mobBurst = stdBurst
	}
	return &PerOperatorRateLimiter{
		std:      make(map[int64]*rate.Limiter),
		mob:      make(map[int64]*rate.Limiter),
		stdRate:  rate.Limit(cfg.PerOperatorQPS),
		stdBurst: stdBurst,
		mobRate:  rate.Limit(mobRate),
		mobBurst: mobBurst,
	}
}

// Allow 对 operatorID 记账一次；mobileKeyword 为 true 时使用更严的手机号形态配额。
func (p *PerOperatorRateLimiter) Allow(operatorID int64, mobileKeyword bool) bool {
	if p == nil || operatorID <= 0 {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if mobileKeyword {
		lim, ok := p.mob[operatorID]
		if !ok {
			lim = rate.NewLimiter(p.mobRate, p.mobBurst)
			p.mob[operatorID] = lim
		}
		return lim.Allow()
	}
	lim, ok := p.std[operatorID]
	if !ok {
		lim = rate.NewLimiter(p.stdRate, p.stdBurst)
		p.std[operatorID] = lim
	}
	return lim.Allow()
}
