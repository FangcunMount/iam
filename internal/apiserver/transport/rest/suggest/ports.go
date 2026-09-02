package suggest

// RateLimiter 按 operator 限流。
type RateLimiter interface {
	Allow(operatorID int64, mobileKeyword bool) bool
}

// RateLimitMetrics REST 限流指标。
type RateLimitMetrics interface {
	RecordRateLimited(mobileKeyword bool)
}
