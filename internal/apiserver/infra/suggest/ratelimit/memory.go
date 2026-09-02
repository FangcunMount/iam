package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const defaultOperatorMapMaxEntries = 10000

type memoryLimiterEntry struct {
	lim        *rate.Limiter
	lastAccess time.Time
}

// MemoryLimiter 进程内按 operator 分桶限流（令牌桶）。
type MemoryLimiter struct {
	mu sync.Mutex

	std map[int64]*memoryLimiterEntry
	mob map[int64]*memoryLimiterEntry

	stdRate  rate.Limit
	stdBurst int
	mobRate  rate.Limit
	mobBurst int

	maxEntries int
}

// NewMemoryLimiter 创建进程内限流器。
func NewMemoryLimiter(cfg Config) *MemoryLimiter {
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
	maxEntries := cfg.OperatorMapMaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultOperatorMapMaxEntries
	}
	return &MemoryLimiter{
		std:        make(map[int64]*memoryLimiterEntry),
		mob:        make(map[int64]*memoryLimiterEntry),
		stdRate:    rate.Limit(cfg.PerOperatorQPS),
		stdBurst:   stdBurst,
		mobRate:    rate.Limit(mobRate),
		mobBurst:   mobBurst,
		maxEntries: maxEntries,
	}
}

// Allow 对 operatorID 记账一次；mobileKeyword 为 true 时使用更严的手机号形态配额。
func (p *MemoryLimiter) Allow(operatorID int64, mobileKeyword bool) bool {
	if p == nil || operatorID <= 0 {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if mobileKeyword {
		return p.allowLocked(p.mob, operatorID, p.mobRate, p.mobBurst, now)
	}
	return p.allowLocked(p.std, operatorID, p.stdRate, p.stdBurst, now)
}

func (p *MemoryLimiter) allowLocked(
	buckets map[int64]*memoryLimiterEntry,
	operatorID int64,
	limRate rate.Limit,
	burst int,
	now time.Time,
) bool {
	ent, ok := buckets[operatorID]
	if !ok {
		p.evictIfNeededLocked(buckets)
		ent = &memoryLimiterEntry{lim: rate.NewLimiter(limRate, burst), lastAccess: now}
		buckets[operatorID] = ent
	} else {
		ent.lastAccess = now
	}
	return ent.lim.Allow()
}

func (p *MemoryLimiter) evictIfNeededLocked(buckets map[int64]*memoryLimiterEntry) {
	if len(buckets) < p.maxEntries {
		return
	}
	var oldestID int64
	var oldest time.Time
	first := true
	for id, ent := range buckets {
		if first || ent.lastAccess.Before(oldest) {
			oldest = ent.lastAccess
			oldestID = id
			first = false
		}
	}
	if !first {
		delete(buckets, oldestID)
	}
}
