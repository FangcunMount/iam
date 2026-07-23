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

	output := opts.String()
	for _, secret := range []string{
		opts.MySQLOptions.Password,
		opts.RedisOptions.Cache.Password,
		opts.IDP.EncryptionKey,
		opts.SMS.Aliyun.AccessKeyID,
		opts.SMS.Aliyun.AccessKeySecret,
		opts.SeedMockAuth.SharedSecret,
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
