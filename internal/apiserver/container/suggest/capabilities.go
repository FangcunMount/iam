package suggest

import appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"

// ApplicationCapabilities contains suggest application collaborators used
// by transports without exposing concrete transport objects from the module.
type ApplicationCapabilities struct {
	Service     appsuggest.ProfileSuggestor
	RateLimit   appsuggest.RateLimitConfig
	Metrics     appsuggest.SuggestMetrics
	RateLimiter appsuggest.RateLimiter
}

// RuntimeCapabilities exposes background collaborators owned by suggest.
type RuntimeCapabilities struct {
	Cleanup func() error
}
