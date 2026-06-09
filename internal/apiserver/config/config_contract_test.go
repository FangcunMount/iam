package config

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
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
				assertEqual(t, "app mode", opts.App.Mode, "development")
				assertEqual(t, "auth issuer", opts.Auth.JWTIssuer, "https://iam.fangcunmount.cn")
				assertEqual(t, "auth access ttl", opts.Auth.AccessTokenTTL, 15*time.Minute)
				assertEqual(t, "auth refresh ttl", opts.Auth.RefreshTokenTTL, 168*time.Hour)
				assertEqual(t, "auth session max ttl", opts.Auth.SessionMaxTTL, 24*time.Hour)
				assertEqual(t, "jwks keys dir", opts.JWKS.KeysDir, "./configs/keys")
				assertEqual(t, "jwks auto init", opts.JWKS.AutoInit, true)
				assertEqual(t, "idp encryption key", opts.IDP.EncryptionKey, "")
				assertEqual(t, "idp wecom agent id", opts.IDP.WeCom.AgentID, "")
				assertEqual(t, "sms provider", opts.SMS.Provider, "log")
				assertEqual(t, "sms otp ttl", opts.SMS.LoginOTPTTL, 5*time.Minute)
				assertEqual(t, "sms cooldown", opts.SMS.LoginOTPSendCooldown, time.Minute)
				assertEqual(t, "sms code length", opts.SMS.LoginOTPCodeLength, 6)
				assertEqual(t, "sms hourly limit", opts.SMS.LoginOTPHourlyLimit, 5)
				assertEqual(t, "sms daily limit", opts.SMS.LoginOTPDailyLimit, 10)
				assertEqual(t, "sms aliyun endpoint", opts.SMS.Aliyun.Endpoint, "dypnsapi.aliyuncs.com")
				assertEqual(t, "sms aliyun code param", opts.SMS.Aliyun.CodeParamName, "code")
				assertEqual(t, "sms aliyun min param", opts.SMS.Aliyun.MinParamName, "min")
				assertEqual(t, "suggest enabled", opts.Suggest.Enable, true)
				assertEqual(t, "suggest data dir", opts.Suggest.DataDir, "./data/suggest")
				assertEqual(t, "suggest delta cron", opts.Suggest.DeltaSyncCron, "")
				assertBoolPtr(t, "suggest snapshot", opts.Suggest.Snapshot, true)
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
				assertEqual(t, "redis cache max active", opts.RedisOptions.Cache.MaxActive, 256)
				assertEqual(t, "redis cache max idle", opts.RedisOptions.Cache.MaxIdle, 40)
				assertEqual(t, "redis cache min idle conns", opts.RedisOptions.Cache.MinIdleConns, 10)
				assertEqual(t, "redis cache logging", opts.RedisOptions.Cache.EnableLogging, false)
				assertEqual(t, "migration database", opts.MigrationOptions.Database, "iam")
				assertEqual(t, "app mode", opts.App.Mode, "production")
				assertEqual(t, "auth issuer", opts.Auth.JWTIssuer, "https://iam.fangcunmount.cn")
				assertEqual(t, "auth audience count", len(opts.Auth.AccessTokenAudience), 2)
				assertEqual(t, "auth session max ttl", opts.Auth.SessionMaxTTL, 24*time.Hour)
				assertEqual(t, "jwks keys dir", opts.JWKS.KeysDir, "/app/data/keys")
				assertEqual(t, "jwks auto init", opts.JWKS.AutoInit, true)
				assertEqual(t, "idp encryption key", opts.IDP.EncryptionKey, "CHANGE_ME_WITH_32_BYTE_BASE64_SECRET")
				assertEqual(t, "idp wecom agent id", opts.IDP.WeCom.AgentID, "")
				assertEqual(t, "sms provider", opts.SMS.Provider, "aliyun")
				assertEqual(t, "sms hourly limit", opts.SMS.LoginOTPHourlyLimit, 5)
				assertEqual(t, "sms daily limit", opts.SMS.LoginOTPDailyLimit, 10)
				assertEqual(t, "sms mq topic", opts.SMS.MQ.Topic, "iam.notify.sms")
				assertEqual(t, "sms aliyun endpoint", opts.SMS.Aliyun.Endpoint, "dypnsapi.aliyuncs.com")
				assertEqual(t, "sms aliyun code param", opts.SMS.Aliyun.CodeParamName, "code")
				assertEqual(t, "sms aliyun min param", opts.SMS.Aliyun.MinParamName, "min")
				assertEqual(t, "suggest enabled", opts.Suggest.Enable, true)
				assertEqual(t, "suggest full cron", opts.Suggest.FullSyncCron, "@every 6h")
				assertEqual(t, "suggest delta cron", opts.Suggest.DeltaSyncCron, "")
				assertBoolPtr(t, "suggest snapshot", opts.Suggest.Snapshot, true)
				assertBoolPtr(t, "debug cache governance enabled", opts.Debug.CacheGovernance.Enabled, false)
				assertBoolPtr(t, "debug cache governance require admin", opts.Debug.CacheGovernance.RequireAdmin, true)
				assertEqual(t, "seed mock enabled", opts.SeedMockAuth.Enabled, true)
				assertEqual(t, "seed mock secret", opts.SeedMockAuth.SharedSecret, "N&#$Xds8sz72s0!8wkzsdcbWCnfJ6S5")
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
