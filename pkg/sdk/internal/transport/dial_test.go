package transport

import (
	"strings"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestFormatServiceConfigDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "zero", in: 0, want: "0s"},
		{name: "milliseconds", in: 100 * time.Millisecond, want: "0.1s"},
		{name: "fractional seconds", in: 1500 * time.Millisecond, want: "1.5s"},
		{name: "sub millisecond", in: 125 * time.Microsecond, want: "0.000125s"},
		{name: "seconds", in: 10 * time.Second, want: "10s"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatServiceConfigDuration(tt.in); got != tt.want {
				t.Fatalf("formatServiceConfigDuration(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildRetryConfigUsesProtoJSONDurations(t *testing.T) {
	t.Parallel()

	retryJSON := buildRetryConfig(&config.RetryConfig{
		Enabled:           true,
		MaxAttempts:       4,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		RetryableCodes:    []string{"UNAVAILABLE"},
	})

	if !strings.Contains(retryJSON, `"initialBackoff": "0.1s"`) {
		t.Fatalf("retry config does not contain protobuf initial backoff: %s", retryJSON)
	}
	if !strings.Contains(retryJSON, `"maxBackoff": "1.5s"`) {
		t.Fatalf("retry config does not contain protobuf max backoff: %s", retryJSON)
	}
}

func TestBuildServiceConfigAcceptedByGRPC(t *testing.T) {
	t.Parallel()

	serviceConfig := BuildServiceConfig(&config.Config{
		Retry: &config.RetryConfig{
			Enabled:           true,
			MaxAttempts:       3,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        10 * time.Second,
			BackoffMultiplier: 2.0,
			RetryableCodes:    []string{"UNAVAILABLE"},
		},
	})

	conn, err := grpc.NewClient(
		"passthrough:///test.server",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient rejected service config %s: %v", serviceConfig, err)
	}
	_ = conn.Close()
}
