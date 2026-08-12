package handler

import (
	"net/url"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	profileLinkQueryDefault        = "default"
	profileLinkQueryIncludeRevoked = "include_revoked"
	profileLinkQueryLegacyActive   = "legacy_active"
	profileLinkQueryBoth           = "both"
)

var profileLinkQueryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "identity",
	Name:      "profile_link_query_total",
	Help:      "ProfileLink list requests by bounded query compatibility mode.",
}, []string{"mode"})

func init() {
	for _, mode := range []string{
		profileLinkQueryDefault,
		profileLinkQueryIncludeRevoked,
		profileLinkQueryLegacyActive,
		profileLinkQueryBoth,
	} {
		profileLinkQueryTotal.WithLabelValues(mode).Add(0)
	}
}

func recordProfileLinkQuery(values url.Values) {
	profileLinkQueryTotal.WithLabelValues(profileLinkQueryMode(values)).Inc()
}

func profileLinkQueryMode(values url.Values) string {
	includeRevoked := values.Has("include_revoked")
	active := values.Has("active")
	switch {
	case includeRevoked && active:
		return profileLinkQueryBoth
	case includeRevoked:
		return profileLinkQueryIncludeRevoked
	case active:
		return profileLinkQueryLegacyActive
	default:
		return profileLinkQueryDefault
	}
}
