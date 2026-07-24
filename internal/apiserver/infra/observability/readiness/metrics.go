package readiness

import (
	readinessapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/readiness"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	componentStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "iam",
		Subsystem: "readiness",
		Name:      "component_status",
		Help:      "Current readiness component status, 1 for ok and 0 otherwise.",
	}, []string{"component"})
	checksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iam",
		Subsystem: "readiness",
		Name:      "checks_total",
		Help:      "Readiness checks by stable result.",
	}, []string{"result"})
)

type Recorder struct{}

func (Recorder) Observe(snapshot readinessapp.Snapshot, ready bool) {
	result := "not_ready"
	if ready {
		result = "ready"
	}
	checksTotal.WithLabelValues(result).Inc()
	for component, state := range snapshot.Components {
		value := 0.0
		if state.Status == readinessapp.StatusOK {
			value = 1
		}
		componentStatus.WithLabelValues(component).Set(value)
	}
}
