package options

import (
	"strings"
	"testing"
	"time"
)

func TestOptionsStringOmitsSecrets(t *testing.T) {
	opts := NewOptions()
	opts.MySQLOptions.Password = "mysql-password-sentinel"
	opts.RedisOptions.Cache.Password = "redis-password-sentinel"
	opts.IDP.EncryptionKey = "idp-encryption-key-sentinel"
	opts.SMS.Aliyun.AccessKeyID = "sms-access-key-id-sentinel"
	opts.SMS.Aliyun.AccessKeySecret = "sms-access-key-secret-sentinel"
	opts.SeedMockAuth.SharedSecret = "seed-shared-secret-sentinel"
	removedValue := "removed-app-value-sentinel"
	opts.RemovedApp.Mode = &removedValue

	output := opts.String()
	for _, secret := range []string{
		opts.MySQLOptions.Password,
		opts.RedisOptions.Cache.Password,
		opts.IDP.EncryptionKey,
		opts.SMS.Aliyun.AccessKeyID,
		opts.SMS.Aliyun.AccessKeySecret,
		opts.SeedMockAuth.SharedSecret,
		removedValue,
	} {
		if strings.Contains(output, secret) {
			t.Fatalf("Options.String() leaked %q: %s", secret, output)
		}
	}
}

func TestOptionsValidateSeedMockAuth(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		secret     string
		wantErr    bool
		errMessage string
	}{
		{name: "disabled without secret", enabled: false},
		{name: "enabled with secret", enabled: true, secret: "test-secret"},
		{
			name:       "enabled without secret",
			enabled:    true,
			wantErr:    true,
			errMessage: "seed_mock_auth.shared_secret is required",
		},
		{
			name:       "enabled with whitespace secret",
			enabled:    true,
			secret:     " \t ",
			wantErr:    true,
			errMessage: "seed_mock_auth.shared_secret is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions()
			opts.GenericServerRunOptions.Mode = "debug"
			opts.SeedMockAuth.Enabled = tt.enabled
			opts.SeedMockAuth.SharedSecret = tt.secret

			errs := opts.Validate()
			if !tt.wantErr {
				if len(errs) != 0 {
					t.Fatalf("Options.Validate() errors = %v, want none", errs)
				}
				return
			}

			for _, err := range errs {
				if strings.Contains(err.Error(), tt.errMessage) {
					return
				}
			}
			t.Fatalf("Options.Validate() errors = %v, want one containing %q", errs, tt.errMessage)
		})
	}
}

func TestOptionsValidatePasswordLockout(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		threshold  int
		duration   time.Duration
		wantErr    bool
		errMessage string
	}{
		{name: "disabled allows zero values"},
		{name: "enabled valid", enabled: true, threshold: 5, duration: 15 * time.Minute},
		{name: "enabled invalid threshold", enabled: true, duration: time.Minute, wantErr: true, errMessage: "threshold"},
		{name: "enabled invalid duration", enabled: true, threshold: 5, wantErr: true, errMessage: "lock_duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions()
			opts.GenericServerRunOptions.Mode = "debug"
			opts.Auth.PasswordLockout = PasswordLockoutOptions{
				Enabled:      tt.enabled,
				Threshold:    tt.threshold,
				LockDuration: tt.duration,
			}
			errs := opts.Validate()
			if !tt.wantErr {
				if len(errs) != 0 {
					t.Fatalf("Options.Validate() errors = %v, want none", errs)
				}
				return
			}
			for _, err := range errs {
				if strings.Contains(err.Error(), tt.errMessage) {
					return
				}
			}
			t.Fatalf("Options.Validate() errors = %v, want one containing %q", errs, tt.errMessage)
		})
	}
}

func TestOptionsDefaultsToReleaseProductionProfile(t *testing.T) {
	opts := NewOptions()
	if err := opts.GenericServerRunOptions.Complete(); err != nil {
		t.Fatal(err)
	}
	profile, err := opts.GenericServerRunOptions.RuntimeProfile()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(profile.ServerMode); got != "release" {
		t.Fatalf("ServerMode = %q, want release", got)
	}
	if got := string(profile.Environment); got != "production" {
		t.Fatalf("Environment = %q, want production", got)
	}
}

func TestOptionsValidateJWKSRotation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Options)
		wantErr string
	}{
		{name: "valid production", mutate: func(o *Options) {
			o.JWKS.KeysDir = "/var/lib/iam/keys"
			o.SMS.Provider = "aliyun"
		}},
		{name: "release requires absolute keys dir", mutate: func(o *Options) {
			o.JWKS.KeysDir = "./keys"
			o.SMS.Provider = "aliyun"
		}, wantErr: "absolute path"},
		{name: "invalid cron", mutate: func(o *Options) {
			o.GenericServerRunOptions.Mode = "debug"
			o.JWKS.Rotation.CheckCron = "not-a-cron"
		}, wantErr: "check_cron"},
		{name: "grace covers access ttl", mutate: func(o *Options) {
			o.GenericServerRunOptions.Mode = "debug"
			o.JWKS.Rotation.GracePeriod = time.Minute
		}, wantErr: "access_token_ttl"},
		{name: "grace shorter than rotation", mutate: func(o *Options) {
			o.GenericServerRunOptions.Mode = "debug"
			o.JWKS.Rotation.GracePeriod = o.JWKS.Rotation.RotationInterval
		}, wantErr: "shorter"},
		{name: "max at least two", mutate: func(o *Options) {
			o.GenericServerRunOptions.Mode = "debug"
			o.JWKS.Rotation.MaxPublishableKey = 1
		}, wantErr: "at least 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions()
			tt.mutate(opts)
			errs := opts.Validate()
			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("Validate() errors = %v", errs)
				}
				return
			}
			for _, err := range errs {
				if strings.Contains(err.Error(), tt.wantErr) {
					return
				}
			}
			t.Fatalf("Validate() errors = %v, want %q", errs, tt.wantErr)
		})
	}
}

func TestOptionsValidateReadiness(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ReadinessOptions)
		wantErr string
	}{
		{name: "valid"},
		{name: "component timeout positive", mutate: func(o *ReadinessOptions) {
			o.ComponentTimeout = 0
		}, wantErr: "component_timeout"},
		{name: "total greater than component", mutate: func(o *ReadinessOptions) {
			o.TotalTimeout = o.ComponentTimeout
		}, wantErr: "total_timeout"},
		{name: "outbox age positive", mutate: func(o *ReadinessOptions) {
			o.OutboxMaxPendingAge = 0
		}, wantErr: "outbox_max_pending_age"},
		{name: "drain delay nonnegative", mutate: func(o *ReadinessOptions) {
			o.DrainDelay = -time.Second
		}, wantErr: "drain_delay"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions()
			opts.GenericServerRunOptions.Mode = "debug"
			if tt.mutate != nil {
				tt.mutate(&opts.Health.Readiness)
			}
			errs := opts.Validate()
			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("Validate() errors = %v", errs)
				}
				return
			}
			for _, err := range errs {
				if strings.Contains(err.Error(), tt.wantErr) {
					return
				}
			}
			t.Fatalf("Validate() errors = %v, want %q", errs, tt.wantErr)
		})
	}
}

func TestOptionsValidateRejectsRemovedAppKeysWithoutValues(t *testing.T) {
	sentinel := "removed-app-sentinel"
	tests := []struct {
		name string
		key  string
		set  func(*RemovedAppOptions)
	}{
		{name: "name", key: "app.name", set: func(o *RemovedAppOptions) { o.Name = &sentinel }},
		{name: "version", key: "app.version", set: func(o *RemovedAppOptions) { o.Version = &sentinel }},
		{name: "mode", key: "app.mode", set: func(o *RemovedAppOptions) { o.Mode = &sentinel }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions()
			tt.set(opts.RemovedApp)
			errs := opts.Validate()
			assertValidationErrorWithoutValue(t, errs, tt.key, sentinel)
		})
	}
}

func TestOptionsValidateRejectsRemovedEnvironmentVariablesWithoutValues(t *testing.T) {
	const sentinel = "removed-env-sentinel"
	for _, key := range []string{
		"IAM_APISERVER_APP_NAME",
		"IAM_APISERVER_APP_VERSION",
		"IAM_APISERVER_APP_MODE",
		"IAM_APISERVER_SERVER_RUN_MODE",
		"IAM_APISERVER_SERVER_NAME",
		"IAM_APISERVER_SERVER_READ_TIMEOUT",
		"IAM_APISERVER_SERVER_WRITE_TIMEOUT",
		"IAM_APISERVER_SUGGEST_DATA_DIR",
		"IAM_APISERVER_SUGGEST_SNAPSHOT",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, sentinel)
			errs := NewOptions().Validate()
			assertValidationErrorWithoutValue(t, errs, key, sentinel)
		})
	}
}

func assertValidationErrorWithoutValue(t *testing.T, errs []error, key, value string) {
	t.Helper()
	for _, err := range errs {
		if strings.Contains(err.Error(), key) {
			if strings.Contains(err.Error(), value) {
				t.Fatalf("validation error leaked %q: %v", value, err)
			}
			return
		}
	}
	t.Fatalf("validation errors = %v, want one containing %q", errs, key)
}
