package process

import (
	"strings"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
)

func TestResolveRuntimeProfileFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		environment genericapiserver.Environment
		production  bool
	}{
		{name: "debug", mode: "debug", environment: genericapiserver.EnvironmentDevelopment},
		{name: "test", mode: "test", environment: genericapiserver.EnvironmentTest},
		{name: "release", mode: "release", environment: genericapiserver.EnvironmentProduction, production: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := apiserveroptions.NewOptions()
			opts.GenericServerRunOptions.Mode = tt.mode
			profile, err := resolveRuntimeProfile(&config.Config{Options: opts})
			if err != nil {
				t.Fatal(err)
			}
			if profile.Environment != tt.environment {
				t.Fatalf("Environment = %q, want %q", profile.Environment, tt.environment)
			}
			if profile.IsProductionLike() != tt.production {
				t.Fatalf("IsProductionLike() = %v, want %v", profile.IsProductionLike(), tt.production)
			}
		})
	}
}

func TestResolveRuntimeProfileDefaultsNilConfigToRelease(t *testing.T) {
	profile, err := resolveRuntimeProfile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if profile.ServerMode != genericapiserver.RuntimeModeRelease ||
		profile.Environment != genericapiserver.EnvironmentProduction {
		t.Fatalf("profile = %#v, want release/production", profile)
	}
}

func TestPrepareRuntimeRejectsInvalidMode(t *testing.T) {
	opts := apiserveroptions.NewOptions()
	opts.GenericServerRunOptions.Mode = "production"
	server := &apiServer{cfg: &config.Config{Options: opts}}

	var state prepareState
	err := (prepareRuntimeStage{server: server}).Run(&state)
	if err == nil || !strings.Contains(err.Error(), "server.mode") {
		t.Fatalf("prepare runtime error = %v, want server.mode validation error", err)
	}
	if state.runtime.profile.ServerMode != "" {
		t.Fatalf("runtime state changed after validation failure: %#v", state.runtime)
	}
}

func TestPrepareRuntimeDegradedStartupMatrix(t *testing.T) {
	for _, tt := range []struct {
		mode      string
		requested bool
		want      bool
	}{
		{mode: "debug", requested: true, want: true},
		{mode: "test", requested: true, want: true},
		{mode: "release", requested: true, want: false},
		{mode: "debug", requested: false, want: false},
		{mode: "test", requested: false, want: false},
		{mode: "release", requested: false, want: false},
	} {
		name := tt.mode
		if tt.requested {
			name += "_requested"
		}
		t.Run(name, func(t *testing.T) {
			opts := apiserveroptions.NewOptions()
			opts.GenericServerRunOptions.Mode = tt.mode
			opts.GenericServerRunOptions.AllowDegradedStartup = tt.requested
			server := &apiServer{cfg: &config.Config{Options: opts}}
			runtime, err := server.prepareRuntime()
			if err != nil {
				t.Fatal(err)
			}
			if runtime.degradedAllowed != tt.want {
				t.Fatalf("degradedAllowed = %v, want %v", runtime.degradedAllowed, tt.want)
			}
		})
	}
}
