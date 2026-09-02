package suggest

import (
	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	suggestmetrics "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/metrics"
	suggestratelimit "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/ratelimit"
)

// ApplicationCapabilities contains suggest application collaborators used by transports.
type ApplicationCapabilities struct {
	Querier     appquery.Querier
	Metrics     suggestmetrics.RateLimitRecorder
	RateLimiter suggestratelimit.Limiter
}

// RuntimeCapabilities exposes background collaborators owned by suggest.
type RuntimeCapabilities struct {
	Cleanup func() error
}
