package jwt

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var missingTokenTypeTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "iam_jwt_missing_token_type_total",
	Help: "Count of verified JWTs that omit token_type during the compatibility window.",
})

var legacyAttributeAuthTimeFallbackTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "iam_jwt_legacy_attribute_auth_time_fallback_total",
	Help: "Count of verified JWTs that recover auth_time from attributes during the compatibility window.",
})
