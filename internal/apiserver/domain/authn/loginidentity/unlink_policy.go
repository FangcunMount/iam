package loginidentity

import (
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

const (
	DefaultRecentAuthWindow = 10 * time.Minute
	DefaultFutureClockSkew  = time.Minute
)

// UnlinkReauthDecision 描述解绑前近期认证检查的结果。
type UnlinkReauthDecision int

const (
	UnlinkReauthOK UnlinkReauthDecision = iota
	UnlinkReauthRequired
	UnlinkReauthInvalidTimestamp
)

// UnlinkReauthRequest 是解绑近期认证策略的确定性输入。
type UnlinkReauthRequest struct {
	Identity               *LoginIdentity
	CurrentLoginIdentityID meta.ID
	AuthenticatedAt        *time.Time
	Now                    time.Time
}

// UnlinkPolicy 封装解绑安全策略。
type UnlinkPolicy struct {
	RecentAuthWindow time.Duration
	FutureClockSkew  time.Duration
}

// DefaultUnlinkPolicy 返回默认解绑策略。
func DefaultUnlinkPolicy() UnlinkPolicy {
	return UnlinkPolicy{
		RecentAuthWindow: DefaultRecentAuthWindow,
		FutureClockSkew:  DefaultFutureClockSkew,
	}
}

// AssessRecentAuthentication 判断解绑是否满足近期认证要求。
func (p UnlinkPolicy) AssessRecentAuthentication(req UnlinkReauthRequest) UnlinkReauthDecision {
	if req.Identity == nil || !p.requiresRecentAuthentication(req) {
		return UnlinkReauthOK
	}
	if !p.hasRecentAuthentication(req.AuthenticatedAt, req.Now) {
		return UnlinkReauthRequired
	}
	return UnlinkReauthOK
}

func (p UnlinkPolicy) requiresRecentAuthentication(req UnlinkReauthRequest) bool {
	if req.Identity == nil {
		return false
	}
	if !req.CurrentLoginIdentityID.IsZero() && req.Identity.ID == req.CurrentLoginIdentityID {
		return true
	}
	switch req.Identity.Provider {
	case ProviderUsername, ProviderPhone:
		return true
	default:
		return false
	}
}

func (p UnlinkPolicy) hasRecentAuthentication(authenticatedAt *time.Time, now time.Time) bool {
	if authenticatedAt == nil || authenticatedAt.IsZero() {
		return false
	}
	window := p.RecentAuthWindow
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
