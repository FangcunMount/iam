package options

import (
	"strings"
	"testing"
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
