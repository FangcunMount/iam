package sdk

import (
	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	"google.golang.org/grpc"
)

type Config = config.Config
type TLSConfig = config.TLSConfig
type RetryConfig = config.RetryConfig
type JWKSConfig = config.JWKSConfig
type KeepaliveConfig = config.KeepaliveConfig
type TokenVerifyConfig = config.TokenVerifyConfig
type CircuitBreakerConfig = config.CircuitBreakerConfig
type ObservabilityConfig = config.ObservabilityConfig
type ServiceAuthConfig = config.ServiceAuthConfig
type ClientOption = config.ClientOption
type MetricsCollector = config.MetricsCollector
type TracingHook = config.TracingHook

func WithUnaryInterceptors(interceptors ...grpc.UnaryClientInterceptor) ClientOption {
	return config.WithUnaryInterceptors(interceptors...)
}

func WithStreamInterceptors(interceptors ...grpc.StreamClientInterceptor) ClientOption {
	return config.WithStreamInterceptors(interceptors...)
}

func WithDialOptions(opts ...grpc.DialOption) ClientOption {
	return config.WithDialOptions(opts...)
}

func WithTracingHook(hook TracingHook) ClientOption {
	return config.WithTracingHook(hook)
}

func WithMetricsCollector(collector MetricsCollector) ClientOption {
	return config.WithMetricsCollector(collector)
}

func WithDisableDefaultInterceptors() ClientOption {
	return config.WithDisableDefaultInterceptors()
}

func ConfigFromEnv() (*Config, error) {
	return config.FromEnv()
}

func ConfigFromEnvWithPrefix(prefix string) (*Config, error) {
	return config.FromEnvWithPrefix(prefix)
}

func ConfigFromViper(getter interface{ Get(string) interface{} }) (*Config, error) {
	return config.FromViper(getter)
}

func ConfigFromViperWithPrefix(getter interface{ Get(string) interface{} }, prefix string) (*Config, error) {
	return config.FromViperWithPrefix(getter, prefix)
}

func NewViperLoader(getter func(key string) interface{}) *config.ViperLoader {
	return config.NewViperLoader(getter)
}

func DefaultConfig() *Config {
	return config.DefaultConfig()
}

func DefaultObservabilityConfig() *ObservabilityConfig {
	return config.DefaultObservabilityConfig()
}
