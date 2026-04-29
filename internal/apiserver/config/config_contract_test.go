package config

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
	"github.com/spf13/viper"
)

func TestAPIServerYAMLConfigMapsToRuntimeOptions(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		assertion func(*testing.T, *apiserveroptions.Options)
	}{
		{
			name: "development",
			file: "configs/apiserver.dev.yaml",
			assertion: func(t *testing.T, opts *apiserveroptions.Options) {
				t.Helper()
				assertEqual(t, "insecure port", opts.InsecureServing.BindPort, 18081)
				assertEqual(t, "secure port", opts.SecureServing.BindPort, 18441)
				assertEqual(t, "grpc port", opts.GRPCOptions.BindPort, 19091)
				assertEqual(t, "grpc healthz port", opts.GRPCOptions.HealthzPort, 19092)
				assertEqual(t, "grpc mtls enabled", opts.GRPCOptions.MTLS.Enabled, true)
				assertEqual(t, "grpc mtls reload interval", opts.GRPCOptions.MTLS.ReloadInterval, 5*time.Minute)
				assertEqual(t, "grpc auth enabled", opts.GRPCOptions.Auth.Enabled, false)
				assertEqual(t, "grpc acl enabled", opts.GRPCOptions.ACL.Enabled, false)
				assertEqual(t, "mysql database", opts.MySQLOptions.Database, "iam")
				assertEqual(t, "redis cache host", opts.RedisOptions.Cache.Host, "127.0.0.1")
				assertEqual(t, "redis cache port", opts.RedisOptions.Cache.Port, 6379)
				assertEqual(t, "migration enabled", opts.MigrationOptions.Enabled, true)
			},
		},
		{
			name: "production",
			file: "configs/apiserver.prod.yaml",
			assertion: func(t *testing.T, opts *apiserveroptions.Options) {
				t.Helper()
				assertEqual(t, "insecure port", opts.InsecureServing.BindPort, 9080)
				assertEqual(t, "secure port", opts.SecureServing.BindPort, 0)
				assertEqual(t, "grpc port", opts.GRPCOptions.BindPort, 9090)
				assertEqual(t, "grpc healthz port", opts.GRPCOptions.HealthzPort, 9091)
				assertEqual(t, "grpc mtls enabled", opts.GRPCOptions.MTLS.Enabled, true)
				assertEqual(t, "grpc acl enabled", opts.GRPCOptions.ACL.Enabled, true)
				assertEqual(t, "grpc acl config file", opts.GRPCOptions.ACL.ConfigFile, "/app/configs/grpc_acl.yaml")
				assertEqual(t, "mysql database", opts.MySQLOptions.Database, "iam")
				assertEqual(t, "redis cache logging", opts.RedisOptions.Cache.EnableLogging, false)
				assertEqual(t, "migration database", opts.MigrationOptions.Database, "iam")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := loadOptionsFromConfigFile(t, tt.file)
			tt.assertion(t, opts)

			cfg, err := CreateConfigFromOptions(opts)
			if err != nil {
				t.Fatalf("CreateConfigFromOptions() error = %v", err)
			}
			if cfg.Options != opts {
				t.Fatalf("Config.Options does not preserve the decoded options pointer")
			}
		})
	}
}

func loadOptionsFromConfigFile(t *testing.T, file string) *apiserveroptions.Options {
	t.Helper()

	opts := apiserveroptions.NewOptions()
	reader := viper.New()
	reader.SetConfigFile(filepath.Join(repoRoot(t), file))
	if err := reader.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig(%s) error = %v", file, err)
	}
	if err := reader.Unmarshal(opts); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", file, err)
	}
	return opts
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}

func assertEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}
