package keyset

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rotationResults = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam",
		Subsystem: "jwks",
		Name:      "rotation_attempts_total",
		Help:      "JWKS rotation attempts by result.",
	}, []string{"result"})
	lastRotationSuccess = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "iam",
		Subsystem: "jwks",
		Name:      "last_rotation_success_unixtime",
		Help:      "Unix timestamp of the last successful JWKS rotation.",
	})
	keyStateCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "iam",
		Subsystem: "jwks",
		Name:      "keys",
		Help:      "JWKS key rows by lifecycle state.",
	}, []string{"state"})
	candidateCleanupFailures = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "iam",
		Subsystem: "jwks",
		Name:      "candidate_cleanup_failures_total",
		Help:      "Failed cleanup attempts for unpublished candidate private keys.",
	})
	materialValidationFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam",
		Subsystem: "jwks",
		Name:      "material_validation_failures_total",
		Help:      "Active private-key material validation failures.",
	}, []string{"reason"})
	lifecycleOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam",
		Subsystem: "jwks",
		Name:      "lifecycle_operations_total",
		Help:      "JWKS lifecycle application operations by operation and result.",
	}, []string{"operation", "result"})
	postCommitFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam",
		Subsystem: "jwks",
		Name:      "post_commit_failures_total",
		Help:      "JWKS post-commit failures by stage.",
	}, []string{"stage"})
)

func recordRotationResult(result string) {
	rotationResults.WithLabelValues(result).Inc()
	if result == "success" {
		lastRotationSuccess.Set(float64(time.Now().Unix()))
	}
}

func setKeyStateCounts(active, grace, retired int64) {
	keyStateCount.WithLabelValues("active").Set(float64(active))
	keyStateCount.WithLabelValues("grace").Set(float64(grace))
	keyStateCount.WithLabelValues("retired").Set(float64(retired))
}

func recordLifecycleOperation(operation, result string) {
	lifecycleOperations.WithLabelValues(operation, result).Inc()
}

func recordPostCommitFailure(stage string) {
	postCommitFailures.WithLabelValues(stage).Inc()
}
