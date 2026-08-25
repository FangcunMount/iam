package native

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	authorizationChecks = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "checks_total",
		Help: "Native authorization decisions by low-cardinality result.",
	}, []string{"result"})
	authorizationLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "check_duration_seconds",
		Help:    "Native authorization decision latency by low-cardinality result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"result"})
	reloads = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "reloads_total",
		Help: "Native authorization snapshot reloads by result.",
	}, []string{"result"})
	reloadLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "reload_duration_seconds",
		Help:    "Native authorization snapshot build and swap latency.",
		Buckets: prometheus.DefBuckets,
	})
	loadedPolicyVersion = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "loaded_policy_version_max",
		Help: "Maximum policy version in the active immutable snapshot.",
	})
	policyVersionLag = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "policy_version_lag_max",
		Help: "Maximum observed event-to-loaded policy version lag without tenant labels.",
	})
)
