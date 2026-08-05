package config

import (
	"crypto/tls"
	"time"
)

const (
	defaultTimeout                      = 30 * time.Second
	defaultDialTimeout                  = 10 * time.Second
	defaultLoadBalancer                 = "round_robin"
	defaultKeepaliveTime                = 5 * time.Minute
	defaultKeepaliveTimeout             = 20 * time.Second
	defaultKeepalivePermitWithoutStream = false
	defaultRetryInitialBackoff          = 100 * time.Millisecond
	defaultRetryMaxBackoff              = 10 * time.Second
	defaultRetryBackoffMultiplier       = 2.0
	defaultJWKSRefreshInterval          = 5 * time.Minute
	defaultJWKSRequestTimeout           = 5 * time.Second
)

// DefaultObservabilityConfig 默认可观测性配置。
func DefaultObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		EnableMetrics:        true,
		EnableTracing:        false,
		EnableCircuitBreaker: true,
		EnableRequestID:      true,
		MetricsNamespace:     "iam",
		MetricsSubsystem:     "sdk",
		ServiceName:          "iam-sdk",
	}
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Timeout:      defaultTimeout,
		DialTimeout:  defaultDialTimeout,
		LoadBalancer: defaultLoadBalancer,
		TLS: &TLSConfig{
			Enabled:    true,
			MinVersion: tls.VersionTLS12,
		},
		Retry: &RetryConfig{
			Enabled:           true,
			MaxAttempts:       3,
			InitialBackoff:    defaultRetryInitialBackoff,
			MaxBackoff:        defaultRetryMaxBackoff,
			BackoffMultiplier: defaultRetryBackoffMultiplier,
			RetryableCodes:    []string{"UNAVAILABLE", "RESOURCE_EXHAUSTED", "ABORTED"},
		},
		Keepalive: &KeepaliveConfig{
			Time:                defaultKeepaliveTime,
			Timeout:             defaultKeepaliveTimeout,
			PermitWithoutStream: defaultKeepalivePermitWithoutStream,
		},
	}
}

// WithDefaults 填充默认值。
func (c *Config) WithDefaults() *Config {
	defaults := DefaultConfig()

	if c.Timeout == 0 {
		c.Timeout = defaults.Timeout
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = defaults.DialTimeout
	}
	if c.LoadBalancer == "" {
		c.LoadBalancer = defaults.LoadBalancer
	}
	if c.TLS == nil {
		c.TLS = defaults.TLS
	}
	if c.Retry == nil {
		c.Retry = defaults.Retry
	}
	if c.Keepalive == nil {
		c.Keepalive = defaults.Keepalive
	}

	return c
}
