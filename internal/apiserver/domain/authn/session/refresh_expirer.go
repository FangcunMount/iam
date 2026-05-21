package session

import "time"

// refreshExpirer 用于计算下一次 refresh token 的领域过期时间。
type refreshExpirer struct {
	lifetime LifetimePolicy
}

// newRefreshExpirer 创建 refreshExpirer。
func newRefreshExpirer(lifetime LifetimePolicy) *refreshExpirer {
	return &refreshExpirer{lifetime: lifetime}
}

// NewRefreshExpirer 创建下一次 refresh token 过期时间计算器。
func NewRefreshExpirer(lifetime LifetimePolicy) RefreshExpirer {
	return newRefreshExpirer(lifetime)
}

// NextRefreshExpiresAt 计算下一次 refresh token 的领域过期时间。
func (r *refreshExpirer) NextRefreshExpiresAt(now time.Time, session *Session) (time.Time, error) {
	return r.lifetime.RefreshTokenExpiresAt(now, session)
}
