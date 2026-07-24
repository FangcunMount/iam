package authz

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var assignmentAuthorizationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "grpc_assignment",
	Name:      "authorization_total",
	Help:      "gRPC assignment request authorization decisions.",
}, []string{"service", "operation", "result"})

func recordAssignmentAuthorization(service, operation, result string) {
	if service == "" {
		service = "unknown"
	}
	assignmentAuthorizationTotal.WithLabelValues(service, operation, result).Inc()
}
