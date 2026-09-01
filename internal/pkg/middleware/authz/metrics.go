package authz

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpAuthorizationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "authz_http",
	Name:      "authorization_total",
	Help:      "AuthZ management HTTP authorization decisions.",
}, []string{"resource", "action", "result"})

func recordHTTPAuthorization(resource, action, result string) {
	httpAuthorizationTotal.WithLabelValues(resource, action, result).Inc()
}
