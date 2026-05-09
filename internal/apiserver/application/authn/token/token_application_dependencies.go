package token

import "time"

// TokenApplicationDependencies 是 TokenApplicationService 的装配依赖。
type TokenApplicationDependencies struct {
	AccessTokenCodec AccessTokenCodec
	TokenStore       Store
	SessionManager   SessionManager
	AccessChecker    SubjectAccessEvaluator
	ClaimMapper      ClaimMapper
	AccessTTL        time.Duration
	RefreshTTL       time.Duration
}
