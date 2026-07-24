package challenge

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var otpVerificationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "iam",
	Subsystem: "authn_otp",
	Name:      "verification_total",
	Help:      "OTP verification attempts by bounded scene and result.",
}, []string{"scene", "result"})

func recordOTPVerification(scene, result string) {
	switch scene {
	case SceneLoginPhoneOTP, SceneLinkPhoneOTP:
	default:
		scene = "unknown"
	}
	otpVerificationTotal.WithLabelValues(scene, result).Inc()
}
