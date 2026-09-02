package suggest

import (
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	suggestratelimit "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/ratelimit"
)

// ApplicationCapabilities contains suggest application collaborators used by transports.
type ApplicationCapabilities struct {
	Service     appsuggest.ProfileSuggestor
	RateLimit   suggestratelimit.Config
	Metrics     appsuggest.RateLimitMetrics
	RateLimiter appsuggest.RateLimiter
}

// RuntimeCapabilities exposes background collaborators owned by suggest.
type RuntimeCapabilities struct {
	Cleanup func() error
}
