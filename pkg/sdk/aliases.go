package sdk

import "github.com/FangcunMount/iam/v2/pkg/sdk/config"

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

var WithUnaryInterceptors = config.WithUnaryInterceptors
var WithStreamInterceptors = config.WithStreamInterceptors
var WithDialOptions = config.WithDialOptions
var WithTracingHook = config.WithTracingHook
var WithMetricsCollector = config.WithMetricsCollector
var WithDisableDefaultInterceptors = config.WithDisableDefaultInterceptors

var ConfigFromEnv = config.FromEnv
var ConfigFromEnvWithPrefix = config.FromEnvWithPrefix
var ConfigFromViper = config.FromViper
var ConfigFromViperWithPrefix = config.FromViperWithPrefix
var NewViperLoader = config.NewViperLoader
var DefaultConfig = config.DefaultConfig
var DefaultObservabilityConfig = config.DefaultObservabilityConfig
