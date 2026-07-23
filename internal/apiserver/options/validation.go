package options

import (
	"errors"
	"os"
	"strings"
)

// Validate 验证命令行参数
func (o *Options) Validate() []error {
	var errs []error

	errs = append(errs, o.GenericServerRunOptions.Validate()...)
	errs = append(errs, o.MySQLOptions.Validate()...)
	errs = append(errs, o.Log.Validate()...)
	errs = append(errs, o.validateRemovedAppOptions()...)
	errs = append(errs, validateRemovedEnvironmentVariables()...)
	if o.SeedMockAuth != nil && o.SeedMockAuth.Enabled && strings.TrimSpace(o.SeedMockAuth.SharedSecret) == "" {
		errs = append(errs, errors.New("seed_mock_auth.shared_secret is required when seed_mock_auth.enabled=true"))
	}
	if o.Auth != nil && o.Auth.PasswordLockout.Enabled {
		if o.Auth.PasswordLockout.Threshold < 1 {
			errs = append(errs, errors.New("auth.password_lockout.threshold must be at least 1 when enabled"))
		}
		if o.Auth.PasswordLockout.LockDuration <= 0 {
			errs = append(errs, errors.New("auth.password_lockout.lock_duration must be positive when enabled"))
		}
	}

	return errs
}

func (o *Options) validateRemovedAppOptions() []error {
	if o == nil || o.RemovedApp == nil {
		return nil
	}
	var errs []error
	for _, removed := range []struct {
		set     bool
		message string
	}{
		{set: o.RemovedApp.Name != nil, message: "app.name has been removed; application name is defined by the entrypoint"},
		{set: o.RemovedApp.Version != nil, message: "app.version has been removed; version comes from build metadata"},
		{set: o.RemovedApp.Mode != nil, message: "app.mode has been removed; use server.mode"},
	} {
		if removed.set {
			errs = append(errs, errors.New(removed.message))
		}
	}
	return errs
}

func validateRemovedEnvironmentVariables() []error {
	var errs []error
	for _, removed := range []struct {
		key     string
		message string
	}{
		{key: "IAM_APISERVER_APP_NAME", message: "IAM_APISERVER_APP_NAME has been removed; application name is defined by the entrypoint"},
		{key: "IAM_APISERVER_APP_VERSION", message: "IAM_APISERVER_APP_VERSION has been removed; version comes from build metadata"},
		{key: "IAM_APISERVER_APP_MODE", message: "IAM_APISERVER_APP_MODE has been removed; use IAM_APISERVER_SERVER_MODE"},
		{key: "IAM_APISERVER_SERVER_RUN_MODE", message: "IAM_APISERVER_SERVER_RUN_MODE has been removed; use IAM_APISERVER_SERVER_MODE"},
		{key: "IAM_APISERVER_SERVER_NAME", message: "IAM_APISERVER_SERVER_NAME has been removed and has no replacement"},
		{key: "IAM_APISERVER_SERVER_READ_TIMEOUT", message: "IAM_APISERVER_SERVER_READ_TIMEOUT has been removed; HTTP read timeout is not configurable"},
		{key: "IAM_APISERVER_SERVER_WRITE_TIMEOUT", message: "IAM_APISERVER_SERVER_WRITE_TIMEOUT has been removed; HTTP write timeout is not configurable"},
	} {
		if _, set := os.LookupEnv(removed.key); set {
			errs = append(errs, errors.New(removed.message))
		}
	}
	return errs
}
