package server

import "testing"

func TestResolveRuntimeProfile(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		mode        RuntimeMode
		environment Environment
		production  bool
		development bool
	}{
		{
			name:        "debug",
			raw:         " Debug ",
			mode:        RuntimeModeDebug,
			environment: EnvironmentDevelopment,
			development: true,
		},
		{
			name:        "test",
			raw:         "TEST",
			mode:        RuntimeModeTest,
			environment: EnvironmentTest,
		},
		{
			name:        "release",
			raw:         " release ",
			mode:        RuntimeModeRelease,
			environment: EnvironmentProduction,
			production:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := ResolveRuntimeProfile(tt.raw)
			if err != nil {
				t.Fatalf("ResolveRuntimeProfile(%q) error = %v", tt.raw, err)
			}
			if profile.ServerMode != tt.mode {
				t.Fatalf("ServerMode = %q, want %q", profile.ServerMode, tt.mode)
			}
			if profile.Environment != tt.environment {
				t.Fatalf("Environment = %q, want %q", profile.Environment, tt.environment)
			}
			if profile.IsProductionLike() != tt.production {
				t.Fatalf("IsProductionLike() = %v, want %v", profile.IsProductionLike(), tt.production)
			}
			if profile.IsDevelopment() != tt.development {
				t.Fatalf("IsDevelopment() = %v, want %v", profile.IsDevelopment(), tt.development)
			}
		})
	}
}

func TestResolveRuntimeProfileRejectsUnsupportedModes(t *testing.T) {
	for _, raw := range []string{"", "production", "development", "local", "staging", "relase"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ResolveRuntimeProfile(raw); err == nil {
				t.Fatalf("ResolveRuntimeProfile(%q) error = nil, want error", raw)
			}
		})
	}
}

func TestRuntimeProfileAllowsDegraded(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mode      RuntimeMode
		requested bool
		want      bool
	}{
		{name: "debug requested", mode: RuntimeModeDebug, requested: true, want: true},
		{name: "test requested", mode: RuntimeModeTest, requested: true, want: true},
		{name: "release requested", mode: RuntimeModeRelease, requested: true, want: false},
		{name: "debug not requested", mode: RuntimeModeDebug, requested: false, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := ResolveRuntimeProfile(string(tt.mode))
			if err != nil {
				t.Fatal(err)
			}
			if got := profile.AllowsDegraded(tt.requested); got != tt.want {
				t.Fatalf("AllowsDegraded(%v) = %v, want %v", tt.requested, got, tt.want)
			}
		})
	}
}
