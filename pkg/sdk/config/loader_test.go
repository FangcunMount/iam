package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type staticGetter map[string]interface{}

func (g staticGetter) Get(key string) interface{} {
	return g[key]
}

func TestFromViperParsesAdvancedSections(t *testing.T) {
	t.Parallel()

	cfg, err := FromViper(staticGetter{
		"iam.endpoint":                             "iam.example.com:8081",
		"iam.timeout":                              "45s",
		"iam.keepalive":                            true,
		"iam.keepalive.time":                       "40s",
		"iam.keepalive.timeout":                    "12s",
		"iam.keepalive.permit_without_stream":      false,
		"iam.circuit_breaker":                      true,
		"iam.circuit_breaker.failure_threshold":    7,
		"iam.circuit_breaker.open_duration":        "1m",
		"iam.circuit_breaker.half_open_requests":   4,
		"iam.circuit_breaker.success_threshold":    3,
		"iam.observability":                        true,
		"iam.observability.enable_metrics":         true,
		"iam.observability.enable_tracing":         true,
		"iam.observability.enable_circuit_breaker": false,
		"iam.observability.enable_request_id":      false,
		"iam.observability.metrics_namespace":      "myapp",
		"iam.observability.metrics_subsystem":      "iam_client",
		"iam.observability.service_name":           "gateway",
		"iam.jwks.url":                             "https://iam.example.com/.well-known/jwks.json",
		"iam.jwks.refresh_interval":                "10m",
		"iam.jwks.request_timeout":                 "2s",
	})
	require.NoError(t, err)
	require.Equal(t, "iam.example.com:8081", cfg.Endpoint)
	require.Equal(t, 45*time.Second, cfg.Timeout)

	require.NotNil(t, cfg.Keepalive)
	require.Equal(t, 40*time.Second, cfg.Keepalive.Time)
	require.Equal(t, 12*time.Second, cfg.Keepalive.Timeout)
	require.False(t, cfg.Keepalive.PermitWithoutStream)

	require.NotNil(t, cfg.CircuitBreaker)
	require.Equal(t, 7, cfg.CircuitBreaker.FailureThreshold)
	require.Equal(t, time.Minute, cfg.CircuitBreaker.OpenDuration)
	require.Equal(t, 4, cfg.CircuitBreaker.HalfOpenRequests)
	require.Equal(t, 3, cfg.CircuitBreaker.SuccessThreshold)

	require.NotNil(t, cfg.Observability)
	require.True(t, cfg.Observability.EnableMetrics)
	require.True(t, cfg.Observability.EnableTracing)
	require.False(t, cfg.Observability.EnableCircuitBreaker)
	require.False(t, cfg.Observability.EnableRequestID)
	require.Equal(t, "myapp", cfg.Observability.MetricsNamespace)
	require.Equal(t, "iam_client", cfg.Observability.MetricsSubsystem)
	require.Equal(t, "gateway", cfg.Observability.ServiceName)

	require.NotNil(t, cfg.JWKS)
	require.Equal(t, "https://iam.example.com/.well-known/jwks.json", cfg.JWKS.URL)
	require.Equal(t, 10*time.Minute, cfg.JWKS.RefreshInterval)
	require.Equal(t, 2*time.Second, cfg.JWKS.RequestTimeout)
}

func TestFromEnvParsesAdvancedSections(t *testing.T) {
	t.Setenv("IAM_ENDPOINT", "iam.example.com:8081")
	t.Setenv("IAM_TIMEOUT", "45s")
	t.Setenv("IAM_KEEPALIVE_ENABLED", "true")
	t.Setenv("IAM_KEEPALIVE_TIME", "40s")
	t.Setenv("IAM_KEEPALIVE_TIMEOUT", "12s")
	t.Setenv("IAM_KEEPALIVE_PERMIT_WITHOUT_STREAM", "false")
	t.Setenv("IAM_CIRCUIT_BREAKER_ENABLED", "true")
	t.Setenv("IAM_CIRCUIT_BREAKER_FAILURE_THRESHOLD", "7")
	t.Setenv("IAM_CIRCUIT_BREAKER_OPEN_DURATION", "1m")
	t.Setenv("IAM_CIRCUIT_BREAKER_HALF_OPEN_REQUESTS", "4")
	t.Setenv("IAM_CIRCUIT_BREAKER_SUCCESS_THRESHOLD", "3")
	t.Setenv("IAM_OBSERVABILITY_ENABLED", "true")
	t.Setenv("IAM_OBSERVABILITY_ENABLE_METRICS", "true")
	t.Setenv("IAM_OBSERVABILITY_ENABLE_TRACING", "true")
	t.Setenv("IAM_OBSERVABILITY_ENABLE_CIRCUIT_BREAKER", "false")
	t.Setenv("IAM_OBSERVABILITY_ENABLE_REQUEST_ID", "false")
	t.Setenv("IAM_OBSERVABILITY_METRICS_NAMESPACE", "myapp")
	t.Setenv("IAM_OBSERVABILITY_METRICS_SUBSYSTEM", "iam_client")
	t.Setenv("IAM_OBSERVABILITY_SERVICE_NAME", "gateway")
	t.Setenv("IAM_JWKS_URL", "https://iam.example.com/.well-known/jwks.json")
	t.Setenv("IAM_JWKS_REFRESH_INTERVAL", "10m")
	t.Setenv("IAM_JWKS_REQUEST_TIMEOUT", "2s")

	cfg, err := FromEnv()
	require.NoError(t, err)
	require.Equal(t, "iam.example.com:8081", cfg.Endpoint)
	require.Equal(t, 45*time.Second, cfg.Timeout)

	require.NotNil(t, cfg.Keepalive)
	require.Equal(t, 40*time.Second, cfg.Keepalive.Time)
	require.Equal(t, 12*time.Second, cfg.Keepalive.Timeout)
	require.False(t, cfg.Keepalive.PermitWithoutStream)

	require.NotNil(t, cfg.CircuitBreaker)
	require.Equal(t, 7, cfg.CircuitBreaker.FailureThreshold)
	require.Equal(t, time.Minute, cfg.CircuitBreaker.OpenDuration)
	require.Equal(t, 4, cfg.CircuitBreaker.HalfOpenRequests)
	require.Equal(t, 3, cfg.CircuitBreaker.SuccessThreshold)

	require.NotNil(t, cfg.Observability)
	require.True(t, cfg.Observability.EnableMetrics)
	require.True(t, cfg.Observability.EnableTracing)
	require.False(t, cfg.Observability.EnableCircuitBreaker)
	require.False(t, cfg.Observability.EnableRequestID)
	require.Equal(t, "myapp", cfg.Observability.MetricsNamespace)
	require.Equal(t, "iam_client", cfg.Observability.MetricsSubsystem)
	require.Equal(t, "gateway", cfg.Observability.ServiceName)

	require.NotNil(t, cfg.JWKS)
	require.Equal(t, "https://iam.example.com/.well-known/jwks.json", cfg.JWKS.URL)
	require.Equal(t, 10*time.Minute, cfg.JWKS.RefreshInterval)
	require.Equal(t, 2*time.Second, cfg.JWKS.RequestTimeout)
}

func TestWithDefaultsFillsTLSButKeepsObservabilityExplicit(t *testing.T) {
	t.Parallel()

	cfg := (&Config{Endpoint: "iam.example.com:8081"}).WithDefaults()
	require.NotNil(t, cfg.TLS)
	require.True(t, cfg.TLS.Enabled)
	require.Nil(t, cfg.Observability)
	require.NotNil(t, cfg.Keepalive)
	require.Equal(t, 5*time.Minute, cfg.Keepalive.Time)
	require.Equal(t, 20*time.Second, cfg.Keepalive.Timeout)
	require.False(t, cfg.Keepalive.PermitWithoutStream)
}

func TestKeepaliveLoadersUseContractDefaultsWhenEnabled(t *testing.T) {
	t.Run("viper", func(t *testing.T) {
		t.Parallel()
		cfg, err := FromViper(staticGetter{"iam.endpoint": "iam.example.com:8081", "iam.keepalive": true})
		require.NoError(t, err)
		require.Equal(t, 5*time.Minute, cfg.Keepalive.Time)
		require.Equal(t, 20*time.Second, cfg.Keepalive.Timeout)
		require.False(t, cfg.Keepalive.PermitWithoutStream)
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("IAM_ENDPOINT", "iam.example.com:8081")
		t.Setenv("IAM_KEEPALIVE_ENABLED", "true")
		cfg, err := FromEnv()
		require.NoError(t, err)
		require.Equal(t, 5*time.Minute, cfg.Keepalive.Time)
		require.Equal(t, 20*time.Second, cfg.Keepalive.Timeout)
		require.False(t, cfg.Keepalive.PermitWithoutStream)
	})
}
