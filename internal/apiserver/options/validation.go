package options

import (
	"errors"
	"strings"
)

// Validate 验证命令行参数
func (o *Options) Validate() []error {
	var errs []error

	errs = append(errs, o.GenericServerRunOptions.Validate()...)
	errs = append(errs, o.MySQLOptions.Validate()...)
	errs = append(errs, o.Log.Validate()...)
	if o.SeedMockAuth != nil && o.SeedMockAuth.Enabled && strings.TrimSpace(o.SeedMockAuth.SharedSecret) == "" {
		errs = append(errs, errors.New("seed_mock_auth.shared_secret is required when seed_mock_auth.enabled=true"))
	}

	return errs
}
