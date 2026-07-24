package transport

import (
	"time"

	"google.golang.org/grpc/codes"
)

// DefaultMethodRetryConfig 默认重试配置。
func DefaultMethodRetryConfig() *MethodRetryConfig {
	return &MethodRetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableCodes:    DefaultRetryableCodes(),
	}
}

// IdempotentRetryableCodes 幂等操作可重试的状态码。
func IdempotentRetryableCodes() []codes.Code {
	return []codes.Code{
		codes.Unavailable,
		codes.ResourceExhausted,
		codes.Aborted,
		codes.DeadlineExceeded,
		codes.Unknown,
		codes.Internal,
	}
}

// NonIdempotentRetryableCodes 非幂等操作可重试的状态码。
func NonIdempotentRetryableCodes() []codes.Code {
	return []codes.Code{codes.Unavailable}
}

// DefaultRetryableCodes 默认可重试的状态码。
func DefaultRetryableCodes() []codes.Code {
	return []codes.Code{
		codes.Unavailable,
		codes.ResourceExhausted,
		codes.Aborted,
		codes.DeadlineExceeded,
	}
}
