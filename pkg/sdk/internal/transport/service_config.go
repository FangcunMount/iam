package transport

import (
	"fmt"
	"strings"
	"time"

	"github.com/FangcunMount/iam/pkg/sdk/config"
)

// BuildServiceConfig 构建 gRPC ServiceConfig
func BuildServiceConfig(cfg *config.Config) string {
	var parts []string

	lb := cfg.LoadBalancer
	if lb == "" {
		lb = "round_robin"
	}
	parts = append(parts, fmt.Sprintf(`"loadBalancingPolicy": "%s"`, lb))

	if cfg.Retry != nil && cfg.Retry.Enabled {
		parts = append(parts, buildRetryConfig(cfg.Retry))
	}

	return "{" + strings.Join(parts, ",") + "}"
}

func buildRetryConfig(retry *config.RetryConfig) string {
	maxAttempts := retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	initialBackoff := retry.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = 100 * time.Millisecond
	}

	maxBackoff := retry.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 10 * time.Second
	}

	multiplier := retry.BackoffMultiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}

	codes := retry.RetryableCodes
	if len(codes) == 0 {
		codes = []string{"UNAVAILABLE", "RESOURCE_EXHAUSTED", "ABORTED"}
	}

	return fmt.Sprintf(`"methodConfig": [{
		"name": [{"service": ""}],
		"retryPolicy": {
			"maxAttempts": %d,
			"initialBackoff": "%s",
			"maxBackoff": "%s",
			"backoffMultiplier": %.1f,
			"retryableStatusCodes": [%s]
		}
	}]`, maxAttempts, formatServiceConfigDuration(initialBackoff), formatServiceConfigDuration(maxBackoff), multiplier, formatCodes(codes))
}

func formatCodes(codes []string) string {
	quoted := make([]string, len(codes))
	for i, c := range codes {
		quoted[i] = fmt.Sprintf(`"%s"`, c)
	}
	return strings.Join(quoted, ",")
}

func formatServiceConfigDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	negative := d < 0
	if negative {
		d = -d
	}

	seconds := d / time.Second
	nanos := d % time.Second
	if nanos == 0 {
		if negative {
			return fmt.Sprintf("-%ds", seconds)
		}
		return fmt.Sprintf("%ds", seconds)
	}

	fraction := strings.TrimRight(fmt.Sprintf("%09d", nanos), "0")
	if negative {
		return fmt.Sprintf("-%d.%ss", seconds, fraction)
	}
	return fmt.Sprintf("%d.%ss", seconds, fraction)
}
