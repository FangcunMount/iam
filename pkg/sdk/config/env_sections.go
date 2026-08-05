package config

import "time"

func (l *EnvLoader) loadKeepalive() *KeepaliveConfig {
	if !l.sectionEnabled("KEEPALIVE_ENABLED", "KEEPALIVE_TIME", "KEEPALIVE_TIMEOUT", "KEEPALIVE_PERMIT_WITHOUT_STREAM") {
		return nil
	}

	return &KeepaliveConfig{
		Time:                l.getDuration("KEEPALIVE_TIME", defaultKeepaliveTime),
		Timeout:             l.getDuration("KEEPALIVE_TIMEOUT", defaultKeepaliveTimeout),
		PermitWithoutStream: l.getBool("KEEPALIVE_PERMIT_WITHOUT_STREAM", defaultKeepalivePermitWithoutStream),
	}
}

func (l *EnvLoader) loadJWKS() *JWKSConfig {
	if !l.sectionEnabled("JWKS_ENABLED", "JWKS_URL", "JWKS_GRPC_ENDPOINT") {
		return nil
	}

	url := l.getString("JWKS_URL", "")
	grpcEndpoint := l.getString("JWKS_GRPC_ENDPOINT", "")
	if url == "" && grpcEndpoint == "" {
		return nil
	}

	return &JWKSConfig{
		URL:             url,
		GRPCEndpoint:    grpcEndpoint,
		RefreshInterval: l.getDuration("JWKS_REFRESH_INTERVAL", 5*time.Minute),
		RequestTimeout:  l.getDuration("JWKS_REQUEST_TIMEOUT", 5*time.Second),
		CacheTTL:        l.getDuration("JWKS_CACHE_TTL", 0),
		FallbackOnError: l.getBool("JWKS_FALLBACK_ON_ERROR", false),
	}
}

func (l *EnvLoader) loadCircuitBreaker() *CircuitBreakerConfig {
	if !l.sectionEnabled(
		"CIRCUIT_BREAKER_ENABLED",
		"CIRCUIT_BREAKER_FAILURE_THRESHOLD",
		"CIRCUIT_BREAKER_OPEN_DURATION",
		"CIRCUIT_BREAKER_HALF_OPEN_REQUESTS",
		"CIRCUIT_BREAKER_SUCCESS_THRESHOLD",
	) {
		return nil
	}

	return &CircuitBreakerConfig{
		FailureThreshold: l.getInt("CIRCUIT_BREAKER_FAILURE_THRESHOLD", 5),
		OpenDuration:     l.getDuration("CIRCUIT_BREAKER_OPEN_DURATION", 30*time.Second),
		HalfOpenRequests: l.getInt("CIRCUIT_BREAKER_HALF_OPEN_REQUESTS", 3),
		SuccessThreshold: l.getInt("CIRCUIT_BREAKER_SUCCESS_THRESHOLD", 2),
	}
}

func (l *EnvLoader) loadObservability() *ObservabilityConfig {
	if !l.sectionEnabled(
		"OBSERVABILITY_ENABLED",
		"OBSERVABILITY_ENABLE_METRICS",
		"OBSERVABILITY_ENABLE_TRACING",
		"OBSERVABILITY_ENABLE_CIRCUIT_BREAKER",
		"OBSERVABILITY_ENABLE_REQUEST_ID",
		"OBSERVABILITY_METRICS_NAMESPACE",
		"OBSERVABILITY_METRICS_SUBSYSTEM",
		"OBSERVABILITY_SERVICE_NAME",
	) {
		return nil
	}

	return &ObservabilityConfig{
		EnableMetrics:        l.getBool("OBSERVABILITY_ENABLE_METRICS", false),
		EnableTracing:        l.getBool("OBSERVABILITY_ENABLE_TRACING", false),
		EnableCircuitBreaker: l.getBool("OBSERVABILITY_ENABLE_CIRCUIT_BREAKER", false),
		EnableRequestID:      l.getBool("OBSERVABILITY_ENABLE_REQUEST_ID", false),
		MetricsNamespace:     l.getString("OBSERVABILITY_METRICS_NAMESPACE", "iam"),
		MetricsSubsystem:     l.getString("OBSERVABILITY_METRICS_SUBSYSTEM", "sdk"),
		ServiceName:          l.getString("OBSERVABILITY_SERVICE_NAME", "iam-sdk"),
	}
}
