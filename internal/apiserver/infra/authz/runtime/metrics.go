package runtime

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	freshnessAge           = promauto.NewGauge(prometheus.GaugeOpts{Namespace: "iam", Subsystem: "authz_native", Name: "freshness_age_seconds", Help: "Age of last durable policy confirmation."})
	expiredChecks          = promauto.NewCounter(prometheus.CounterOpts{Namespace: "iam", Subsystem: "authz_native", Name: "expired_checks_total", Help: "Queries rejected due to unavailable policy."})
	versionCheckFailures   = promauto.NewCounter(prometheus.CounterOpts{Namespace: "iam", Subsystem: "authz_native", Name: "version_check_failures_total", Help: "Durable version checks that failed."})
	subscriptionRegistered = promauto.NewGauge(prometheus.GaugeOpts{Namespace: "iam", Subsystem: "authz_native", Name: "subscription_registered", Help: "Policy event subscription registered, not a connectivity proof."})
	authorizationChecks    = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "checks_total",
		Help: "Authorization snapshot decisions by low-cardinality result.",
	}, []string{"result"})
	authorizationLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "check_duration_seconds",
		Help:    "Authorization snapshot decision latency by low-cardinality result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"result"})
	reloads = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "reloads_total",
		Help: "Authorization snapshot reloads by result.",
	}, []string{"result"})
	reloadLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "iam", Subsystem: "authz_native", Name: "reload_duration_seconds",
		Help:    "Authorization snapshot build and swap latency.",
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
