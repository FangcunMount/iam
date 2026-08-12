package rest

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	debugModulesViewCanonical = "canonical"
	debugModulesViewLegacy    = "legacy"
	debugModulesViewCombined  = "combined"
)

var debugModulesRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "runtime",
	Name:      "debug_modules_requests_total",
	Help:      "Debug module status requests by bounded response view.",
}, []string{"view"})

func init() {
	for _, view := range []string{
		debugModulesViewCanonical,
		debugModulesViewLegacy,
		debugModulesViewCombined,
	} {
		debugModulesRequestsTotal.WithLabelValues(view).Add(0)
	}
}

func recordDebugModulesRequest(view string) {
	debugModulesRequestsTotal.WithLabelValues(view).Inc()
}
