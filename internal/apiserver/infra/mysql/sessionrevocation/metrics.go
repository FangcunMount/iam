package sessionrevocation

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	taskCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "iam",
		Subsystem: "identity_session_revocation",
		Name:      "tasks",
		Help:      "Session revocation tasks by durable status.",
	}, []string{"status"})
	oldestTaskAge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "iam",
		Subsystem: "identity_session_revocation",
		Name:      "oldest_seconds",
		Help:      "Age in seconds of the oldest unfinished session revocation task.",
	})
)

func recordTaskState(counts map[string]int64, oldestSeconds float64) {
	for _, status := range []string{StatusPending, StatusProcessing, StatusFailed, StatusCompleted} {
		taskCount.WithLabelValues(status).Set(float64(counts[status]))
	}
	if oldestSeconds < 0 {
		oldestSeconds = 0
	}
	oldestTaskAge.Set(oldestSeconds)
}
