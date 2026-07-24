package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
	"github.com/spf13/pflag"
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
				assertEqual(t, "server mode", opts.GenericServerRunOptions.Mode, "debug")
				assertEqual(t, "auth issuer", opts.Auth.JWTIssuer, "https://iam.fangcunmount.cn")
				assertEqual(t, "auth access ttl", opts.Auth.AccessTokenTTL, 15*time.Minute)
				assertEqual(t, "auth refresh ttl", opts.Auth.RefreshTokenTTL, 168*time.Hour)
				assertEqual(t, "auth session max ttl", opts.Auth.SessionMaxTTL, 24*time.Hour)
				assertEqual(t, "password lockout enabled", opts.Auth.PasswordLockout.Enabled, false)
				assertEqual(t, "password lockout threshold", opts.Auth.PasswordLockout.Threshold, 5)
				assertEqual(t, "password lockout duration", opts.Auth.PasswordLockout.LockDuration, 15*time.Minute)
				assertEqual(t, "jwks keys dir", opts.JWKS.KeysDir, "./configs/keys")
				assertEqual(t, "jwks auto init", opts.JWKS.AutoInit, true)
				assertEqual(t, "jwks rotation automatic", opts.JWKS.Rotation.AutomaticEnabled, false)
				assertEqual(t, "jwks rotation cron", opts.JWKS.Rotation.CheckCron, "@every 1h")
				assertEqual(t, "jwks rotation interval", opts.JWKS.Rotation.RotationInterval, 720*time.Hour)
				assertEqual(t, "jwks rotation grace", opts.JWKS.Rotation.GracePeriod, 168*time.Hour)
				assertEqual(t, "jwks rotation max", opts.JWKS.Rotation.MaxPublishableKey, 3)
				assertEqual(t, "idp encryption key", opts.IDP.EncryptionKey, "")
				assertEqual(t, "idp wecom agent id", opts.IDP.WeCom.AgentID, "")
				assertEqual(t, "sms provider", opts.SMS.Provider, "log")
				assertEqual(t, "sms otp ttl", opts.SMS.LoginOTPTTL, 5*time.Minute)
				assertEqual(t, "sms cooldown", opts.SMS.LoginOTPSendCooldown, time.Minute)
				assertEqual(t, "sms code length", opts.SMS.LoginOTPCodeLength, 6)
				assertEqual(t, "sms max attempts", opts.SMS.LoginOTPMaxAttempts, 5)
				assertEqual(t, "sms hourly limit", opts.SMS.LoginOTPHourlyLimit, 5)
				assertEqual(t, "sms daily limit", opts.SMS.LoginOTPDailyLimit, 10)
				assertEqual(t, "sms aliyun endpoint", opts.SMS.Aliyun.Endpoint, "dypnsapi.aliyuncs.com")
				assertEqual(t, "sms aliyun code param", opts.SMS.Aliyun.CodeParamName, "code")
				assertEqual(t, "sms aliyun min param", opts.SMS.Aliyun.MinParamName, "min")
				assertEqual(t, "suggest enabled", opts.Suggest.Enable, true)
				assertEqual(t, "suggest delta cron", opts.Suggest.DeltaSyncCron, "")
				assertEqual(t, "session revocation poll", opts.Identity.SessionRevocation.PollInterval, 2*time.Second)
				assertEqual(t, "session revocation batch", opts.Identity.SessionRevocation.BatchSize, 50)
				assertEqual(t, "readiness component timeout", opts.Health.Readiness.ComponentTimeout, 500*time.Millisecond)
				assertEqual(t, "readiness total timeout", opts.Health.Readiness.TotalTimeout, 2*time.Second)
				assertEqual(t, "readiness drain delay", opts.Health.Readiness.DrainDelay, time.Duration(0))
				assertBoolPtr(t, "debug cache governance enabled", opts.Debug.CacheGovernance.Enabled, true)
				assertBoolPtr(t, "debug cache governance require admin", opts.Debug.CacheGovernance.RequireAdmin, false)
				assertEqual(t, "seed mock disabled by default", opts.SeedMockAuth.Enabled, false)
				assertEqual(t, "events catalog path", opts.Events.CatalogPath, "configs/events.yaml")
				assertEqual(t, "events relay interval", opts.Events.OutboxRelayInterval, 2*time.Second)
				assertEqual(t, "events relay batch size", opts.Events.OutboxRelayBatchSize, 50)
				assertEqual(t, "events relay retry delay", opts.Events.OutboxRelayRetryDelay, 10*time.Second)
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
				assertEqual(t, "mysql max open connections", opts.MySQLOptions.MaxOpenConnections, 80)
				assertEqual(t, "mysql max idle connections", opts.MySQLOptions.MaxIdleConnections, 20)
				assertEqual(t, "mysql connection lifetime", opts.MySQLOptions.MaxConnectionLifeTime, time.Hour)
				assertEqual(t, "mysql log level", opts.MySQLOptions.LogLevel, 1)
				assertEqual(t, "log format", opts.Log.Format, "json")
				assertEqual(t, "log color", opts.Log.EnableColor, false)
				assertEqual(t, "log development", opts.Log.Development, false)
				assertEqual(t, "redis cache max active", opts.RedisOptions.Cache.MaxActive, 256)
				assertEqual(t, "redis cache max idle", opts.RedisOptions.Cache.MaxIdle, 40)
				assertEqual(t, "redis cache min idle conns", opts.RedisOptions.Cache.MinIdleConns, 10)
				assertEqual(t, "redis cache logging", opts.RedisOptions.Cache.EnableLogging, false)
				assertEqual(t, "migration database", opts.MigrationOptions.Database, "iam")
				assertEqual(t, "server mode", opts.GenericServerRunOptions.Mode, "release")
				assertEqual(t, "auth issuer", opts.Auth.JWTIssuer, "https://iam.fangcunmount.cn")
				assertEqual(t, "auth audience count", len(opts.Auth.AccessTokenAudience), 2)
				assertEqual(t, "auth session max ttl", opts.Auth.SessionMaxTTL, 24*time.Hour)
				assertEqual(t, "password lockout enabled", opts.Auth.PasswordLockout.Enabled, true)
				assertEqual(t, "password lockout threshold", opts.Auth.PasswordLockout.Threshold, 5)
				assertEqual(t, "password lockout duration", opts.Auth.PasswordLockout.LockDuration, 15*time.Minute)
				assertEqual(t, "jwks keys dir", opts.JWKS.KeysDir, "/app/data/keys")
				assertEqual(t, "jwks auto init", opts.JWKS.AutoInit, true)
				assertEqual(t, "jwks rotation automatic", opts.JWKS.Rotation.AutomaticEnabled, true)
				assertEqual(t, "jwks rotation cron", opts.JWKS.Rotation.CheckCron, "@every 1h")
				assertEqual(t, "jwks rotation interval", opts.JWKS.Rotation.RotationInterval, 720*time.Hour)
				assertEqual(t, "jwks rotation grace", opts.JWKS.Rotation.GracePeriod, 168*time.Hour)
				assertEqual(t, "jwks rotation max", opts.JWKS.Rotation.MaxPublishableKey, 3)
				assertEqual(t, "mysql password", opts.MySQLOptions.Password, "")
				assertEqual(t, "idp encryption key", opts.IDP.EncryptionKey, "")
				assertEqual(t, "idp wecom agent id", opts.IDP.WeCom.AgentID, "")
				assertEqual(t, "sms provider", opts.SMS.Provider, "aliyun")
				assertEqual(t, "sms max attempts", opts.SMS.LoginOTPMaxAttempts, 5)
				assertEqual(t, "sms hourly limit", opts.SMS.LoginOTPHourlyLimit, 5)
				assertEqual(t, "sms daily limit", opts.SMS.LoginOTPDailyLimit, 10)
				assertEqual(t, "sms mq topic", opts.SMS.MQ.Topic, "iam.notify.sms")
				assertEqual(t, "sms aliyun endpoint", opts.SMS.Aliyun.Endpoint, "dypnsapi.aliyuncs.com")
				assertEqual(t, "sms aliyun code param", opts.SMS.Aliyun.CodeParamName, "code")
				assertEqual(t, "sms aliyun min param", opts.SMS.Aliyun.MinParamName, "min")
				assertEqual(t, "suggest enabled", opts.Suggest.Enable, true)
				assertEqual(t, "suggest full cron", opts.Suggest.FullSyncCron, "@every 6h")
				assertEqual(t, "suggest delta cron", opts.Suggest.DeltaSyncCron, "")
				assertEqual(t, "session revocation retry max", opts.Identity.SessionRevocation.RetryMaxDelay, 5*time.Minute)
				assertEqual(t, "readiness outbox age", opts.Health.Readiness.OutboxMaxPendingAge, 5*time.Minute)
				assertEqual(t, "readiness drain delay", opts.Health.Readiness.DrainDelay, 5*time.Second)
				assertBoolPtr(t, "debug cache governance enabled", opts.Debug.CacheGovernance.Enabled, false)
				assertBoolPtr(t, "debug cache governance require admin", opts.Debug.CacheGovernance.RequireAdmin, true)
				assertEqual(t, "seed mock disabled by default", opts.SeedMockAuth.Enabled, false)
				assertEqual(t, "seed mock secret", opts.SeedMockAuth.SharedSecret, "")
				assertEqual(t, "events catalog path", opts.Events.CatalogPath, "/app/configs/events.yaml")
				assertEqual(t, "events relay interval", opts.Events.OutboxRelayInterval, 2*time.Second)
				assertEqual(t, "events relay batch size", opts.Events.OutboxRelayBatchSize, 100)
				assertEqual(t, "events relay retry delay", opts.Events.OutboxRelayRetryDelay, 10*time.Second)
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

func TestAPIServerYAMLDoesNotContainRemovedRuntimeKeys(t *testing.T) {
	removedKeys := []string{
		"app.name",
		"app.version",
		"app.mode",
		"server.run-mode",
		"server.name",
		"server.read-timeout",
		"server.write-timeout",
		"suggest.data_dir",
		"suggest.snapshot",
	}
	for _, file := range []string{"configs/apiserver.dev.yaml", "configs/apiserver.prod.yaml"} {
		t.Run(file, func(t *testing.T) {
			reader := viper.New()
			reader.SetConfigFile(filepath.Join(repoRoot(t), file))
			if err := reader.ReadInConfig(); err != nil {
				t.Fatal(err)
			}
			for _, key := range removedKeys {
				if reader.IsSet(key) {
					t.Fatalf("%s contains removed key %s", file, key)
				}
			}
		})
	}
}

func TestRemovedRuntimeYAMLKeysDecodeIntoValidationTombstones(t *testing.T) {
	tests := []struct {
		name string
		key  string
		yaml string
	}{
		{name: "app name", key: "app.name", yaml: "app:\n  name: iam\n"},
		{name: "app version", key: "app.version", yaml: "app:\n  version: 1.0.0\n"},
		{name: "app mode", key: "app.mode", yaml: "app:\n  mode: development\n"},
		{name: "server run mode", key: "server.run-mode", yaml: "server:\n  run-mode: debug\n"},
		{name: "server name", key: "server.name", yaml: "server:\n  name: iam\n"},
		{name: "server read timeout", key: "server.read-timeout", yaml: "server:\n  read-timeout: 60\n"},
		{name: "server write timeout", key: "server.write-timeout", yaml: "server:\n  write-timeout: 60\n"},
		{name: "suggest data dir", key: "suggest.data_dir", yaml: "suggest:\n  data_dir: /tmp/private\n"},
		{name: "suggest snapshot", key: "suggest.snapshot", yaml: "suggest:\n  snapshot: true\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := apiserveroptions.NewOptions()
			reader := viper.New()
			reader.SetConfigType("yaml")
			if err := reader.ReadConfig(strings.NewReader(tt.yaml)); err != nil {
				t.Fatal(err)
			}
			if err := reader.Unmarshal(opts); err != nil {
				t.Fatal(err)
			}
			for _, err := range opts.Validate() {
				if strings.Contains(err.Error(), tt.key) {
					return
				}
			}
			t.Fatalf("Validate() errors = %v, want removed key %s", opts.Validate(), tt.key)
		})
	}
}

func TestServerModeConfigurationPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		envMode  string
		flagMode string
		wantMode string
	}{
		{name: "yaml beats unchanged flag default", wantMode: "debug"},
		{name: "environment beats yaml", envMode: "test", wantMode: "test"},
		{name: "changed flag beats environment", envMode: "test", flagMode: "release", wantMode: "release"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envMode != "" {
				t.Setenv("IAM_APISERVER_SERVER_MODE", tt.envMode)
			}
			flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
			mode := flags.String("server.mode", "release", "")
			if tt.flagMode != "" {
				if err := flags.Set("server.mode", tt.flagMode); err != nil {
					t.Fatal(err)
				}
			}

			reader := viper.New()
			reader.SetConfigType("yaml")
			reader.SetEnvPrefix("IAM_APISERVER")
			reader.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
			reader.AutomaticEnv()
			if err := reader.ReadConfig(strings.NewReader("server:\n  mode: debug\n")); err != nil {
				t.Fatal(err)
			}
			if err := reader.BindPFlag("server.mode", flags.Lookup("server.mode")); err != nil {
				t.Fatal(err)
			}

			opts := apiserveroptions.NewOptions()
			if err := reader.Unmarshal(opts); err != nil {
				t.Fatal(err)
			}
			if opts.GenericServerRunOptions.Mode != tt.wantMode {
				t.Fatalf("Mode = %q, want %q (flag value %q)", opts.GenericServerRunOptions.Mode, tt.wantMode, *mode)
			}
		})
	}
}

func TestPasswordLockoutEnvironmentOverridesYAML(t *testing.T) {
	t.Setenv("IAM_APISERVER_AUTH_PASSWORD_LOCKOUT_ENABLED", "true")
	t.Setenv("IAM_APISERVER_AUTH_PASSWORD_LOCKOUT_THRESHOLD", "7")
	t.Setenv("IAM_APISERVER_AUTH_PASSWORD_LOCKOUT_LOCK_DURATION", "20m")

	reader := viper.New()
	reader.SetConfigFile(filepath.Join(repoRoot(t), "configs/apiserver.dev.yaml"))
	reader.SetEnvPrefix("IAM_APISERVER")
	reader.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	reader.AutomaticEnv()
	if err := reader.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	opts := apiserveroptions.NewOptions()
	if err := reader.Unmarshal(opts); err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "password lockout enabled", opts.Auth.PasswordLockout.Enabled, true)
	assertEqual(t, "password lockout threshold", opts.Auth.PasswordLockout.Threshold, 7)
	assertEqual(t, "password lockout duration", opts.Auth.PasswordLockout.LockDuration, 20*time.Minute)
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

func assertBoolPtr(t *testing.T, label string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %v", label, want)
	}
	if *got != want {
		t.Fatalf("%s = %v, want %v", label, *got, want)
	}
}
