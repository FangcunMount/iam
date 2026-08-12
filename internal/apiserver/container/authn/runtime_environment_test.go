package authn

import (
	"testing"

	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
)

func TestShouldAutoInitializeJWKS(t *testing.T) {
	for _, tt := range []struct {
		name        string
		environment genericapiserver.Environment
		configured  bool
		want        bool
	}{
		{name: "development fallback", environment: genericapiserver.EnvironmentDevelopment, want: true},
		{name: "test without config", environment: genericapiserver.EnvironmentTest, want: false},
		{name: "production without config", environment: genericapiserver.EnvironmentProduction, want: false},
		{name: "test explicitly enabled", environment: genericapiserver.EnvironmentTest, configured: true, want: true},
		{name: "production explicitly enabled", environment: genericapiserver.EnvironmentProduction, configured: true, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAutoInitializeJWKS(tt.environment, tt.configured); got != tt.want {
				t.Fatalf("shouldAutoInitializeJWKS(%q, %v) = %v, want %v", tt.environment, tt.configured, got, tt.want)
			}
		})
	}
}
