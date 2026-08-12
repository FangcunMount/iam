package options

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	compatibilityKeyLoaderPlaceholderTenantID = "suggest.loader_placeholder_tenant_id"
	compatibilityKeySMSMQTopic                = "sms.mq.topic"
)

var compatibilityConfigObservations = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "config",
	Name:      "compatibility_key_observations_total",
	Help:      "Process-start observations for bounded compatibility configuration keys.",
}, []string{"key", "state"})

var compatibilityConfigKeys = []string{
	compatibilityKeyLoaderPlaceholderTenantID,
	compatibilityKeySMSMQTopic,
}

func init() {
	for _, key := range compatibilityConfigKeys {
		compatibilityConfigObservations.WithLabelValues(key, "present").Add(0)
		compatibilityConfigObservations.WithLabelValues(key, "absent").Add(0)
	}
}

// ObserveCompatibilityConfigKeys records only fixed key names and presence.
// It never reads or records configuration values.
func ObserveCompatibilityConfigKeys(isSet func(string) bool) []string {
	present := make([]string, 0, len(compatibilityConfigKeys))
	for _, key := range compatibilityConfigKeys {
		state := "absent"
		if isSet != nil && isSet(key) {
			state = "present"
			present = append(present, key)
		}
		compatibilityConfigObservations.WithLabelValues(key, state).Inc()
	}
	return present
}
