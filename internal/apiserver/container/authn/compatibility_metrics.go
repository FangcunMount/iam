package authn

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	smsPublisherCatalog = "catalog"
	smsPublisherLegacy  = "legacy"
)

var smsPublisherSelections = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "authn_sms",
	Name:      "publisher_selections_total",
	Help:      "SMS publisher selections by bounded catalog or compatibility mode.",
}, []string{"mode"})

func init() {
	smsPublisherSelections.WithLabelValues(smsPublisherCatalog).Add(0)
	smsPublisherSelections.WithLabelValues(smsPublisherLegacy).Add(0)
}

func recordSMSPublisherSelection(mode string) {
	smsPublisherSelections.WithLabelValues(mode).Inc()
}
