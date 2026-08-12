package sdk_test

import (
	"testing"

	"github.com/FangcunMount/iam/v3/pkg/sdk"
)

func TestDefaultConfigReturnsIndependentMutableValues(t *testing.T) {
	first := sdk.DefaultConfig()
	first.Retry.MaxAttempts = 99
	first.Retry.RetryableCodes[0] = "MUTATED"
	first.TLS.Enabled = false

	second := sdk.DefaultConfig()
	if second.Retry.MaxAttempts == 99 {
		t.Fatal("retry config mutation leaked into a later DefaultConfig call")
	}
	if second.Retry.RetryableCodes[0] == "MUTATED" {
		t.Fatal("retryable codes mutation leaked into a later DefaultConfig call")
	}
	if !second.TLS.Enabled {
		t.Fatal("TLS config mutation leaked into a later DefaultConfig call")
	}
}

func TestDefaultObservabilityConfigReturnsIndependentValues(t *testing.T) {
	first := sdk.DefaultObservabilityConfig()
	first.EnableMetrics = false
	first.ServiceName = "mutated"

	second := sdk.DefaultObservabilityConfig()
	if !second.EnableMetrics || second.ServiceName == "mutated" {
		t.Fatal("observability mutation leaked into a later default call")
	}
}
