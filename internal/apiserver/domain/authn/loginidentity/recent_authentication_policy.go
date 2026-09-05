package loginidentity

import "time"

// RecentAuthenticationPolicy 判断原始认证时间是否足够新，供绑定与敏感解绑共用。
// 调用方只能传入可信认证上下文中的时间，不能使用请求时间或令牌刷新时间。
type RecentAuthenticationPolicy struct {
	Window          time.Duration
	FutureClockSkew time.Duration
}

// Allows 要求认证时间存在、在窗口内，且不超出允许的未来时钟偏差。
func (p RecentAuthenticationPolicy) Allows(authenticatedAt *time.Time, now time.Time) bool {
	if authenticatedAt == nil || authenticatedAt.IsZero() {
		return false
	}
	window := p.Window
	if window <= 0 {
		window = DefaultRecentAuthWindow
	}
	skew := p.FutureClockSkew
	if skew <= 0 {
		skew = DefaultFutureClockSkew
	}
	now = now.UTC()
	authAt := authenticatedAt.UTC()
	if authAt.After(now.Add(skew)) {
		return false
	}
	return now.Sub(authAt) <= window
}
