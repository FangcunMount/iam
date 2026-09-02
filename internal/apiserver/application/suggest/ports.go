package suggest

import "context"

// ProfileSuggestor 是 transport 暴露的查询用例端口。
type ProfileSuggestor interface {
	SuggestProfile(ctx context.Context, req SuggestProfileRequest) ([]ProfileSuggestItem, error)
}

// RateLimiter 按 operator 限流。
type RateLimiter interface {
	Allow(operatorID int64, mobileKeyword bool) bool
}

// RateLimitMetrics REST 限流指标。
type RateLimitMetrics interface {
	RecordRateLimited(mobileKeyword bool)
}
