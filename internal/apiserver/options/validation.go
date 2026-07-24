package options

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/robfig/cron/v3"
)

// Validate 验证命令行参数
func (o *Options) Validate() []error {
	var errs []error

	errs = append(errs, o.GenericServerRunOptions.Validate()...)
	errs = append(errs, o.MySQLOptions.Validate()...)
	errs = append(errs, o.Log.Validate()...)
	errs = append(errs, o.validateRemovedAppOptions()...)
	errs = append(errs, o.validateRemovedSuggestOptions()...)
	errs = append(errs, validateRemovedEnvironmentVariables()...)
	errs = append(errs, o.validateProductionLogging()...)
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
	if o.SMS != nil {
		if o.SMS.LoginOTPMaxAttempts < 1 {
			errs = append(errs, errors.New("sms.login_otp_max_attempts must be at least 1"))
		}
		if o.GenericServerRunOptions != nil {
			if profile, err := o.GenericServerRunOptions.RuntimeProfile(); err == nil &&
				profile.IsProductionLike() &&
				strings.EqualFold(strings.TrimSpace(o.SMS.Provider), "log") {
				errs = append(errs, errors.New("sms.provider=log is not allowed in release mode"))
			}
		}
	}
	if o.Identity != nil {
		revocation := o.Identity.SessionRevocation
		if revocation.PollInterval <= 0 {
			errs = append(errs, errors.New("identity.session_revocation.poll_interval must be positive"))
		}
		if revocation.BatchSize <= 0 {
			errs = append(errs, errors.New("identity.session_revocation.batch_size must be positive"))
		}
		if revocation.RetryBaseDelay <= 0 {
			errs = append(errs, errors.New("identity.session_revocation.retry_base_delay must be positive"))
		}
		if revocation.RetryMaxDelay < revocation.RetryBaseDelay {
			errs = append(errs, errors.New("identity.session_revocation.retry_max_delay must not be shorter than retry_base_delay"))
		}
		if revocation.StaleProcessingAfter <= 0 {
			errs = append(errs, errors.New("identity.session_revocation.stale_processing_after must be positive"))
		}
	}
	if o.Health != nil {
		readiness := o.Health.Readiness
		if readiness.ComponentTimeout <= 0 {
			errs = append(errs, errors.New("health.readiness.component_timeout must be positive"))
		}
		if readiness.TotalTimeout <= readiness.ComponentTimeout {
			errs = append(errs, errors.New("health.readiness.total_timeout must be greater than component_timeout"))
		}
		if readiness.OutboxMaxPendingAge <= 0 {
			errs = append(errs, errors.New("health.readiness.outbox_max_pending_age must be positive"))
		}
		if readiness.DrainDelay < 0 {
			errs = append(errs, errors.New("health.readiness.drain_delay must not be negative"))
		}
	}
	errs = append(errs, o.validateJWKSOptions()...)

	return errs
}

func (o *Options) validateProductionLogging() []error {
	if o == nil || o.GenericServerRunOptions == nil {
		return nil
	}
	profile, err := o.GenericServerRunOptions.RuntimeProfile()
	if err != nil || !profile.IsProductionLike() {
		return nil
	}

	var errs []error
	if o.MySQLOptions != nil && o.MySQLOptions.LogLevel != 1 {
		errs = append(errs, errors.New("mysql.log-level must be 1 in release mode"))
	}
	if o.Log == nil {
		return append(errs, errors.New("log configuration is required in release mode"))
	}
	if !strings.EqualFold(strings.TrimSpace(o.Log.Format), "json") {
		errs = append(errs, errors.New("log.format must be json in release mode"))
	}
	if o.Log.EnableColor {
		errs = append(errs, errors.New("log.enable-color must be false in release mode"))
	}
	if o.Log.Development {
		errs = append(errs, errors.New("log.development must be false in release mode"))
	}
	return errs
}

func (o *Options) validateRemovedSuggestOptions() []error {
	if o == nil || o.Suggest == nil {
		return nil
	}
	var errs []error
	if o.Suggest.RemovedDataDir != nil {
		errs = append(errs, errors.New("suggest.data_dir has been removed; suggest indexes are memory-only"))
	}
	if o.Suggest.RemovedSnapshot != nil {
		errs = append(errs, errors.New("suggest.snapshot has been removed; suggest indexes are not persisted to files"))
	}
	return errs
}

func (o *Options) validateJWKSOptions() []error {
	if o == nil || o.JWKS == nil {
		return nil
	}
	var errs []error
	rotation := o.JWKS.Rotation
	if rotation.RotationInterval <= 0 {
		errs = append(errs, errors.New("jwks.rotation.rotation_interval must be positive"))
	}
	if rotation.GracePeriod <= 0 {
		errs = append(errs, errors.New("jwks.rotation.grace_period must be positive"))
	}
	if rotation.RotationInterval > 0 && rotation.GracePeriod >= rotation.RotationInterval {
		errs = append(errs, errors.New("jwks.rotation.grace_period must be shorter than rotation_interval"))
	}
	if o.Auth != nil && rotation.GracePeriod < o.Auth.AccessTokenTTL {
		errs = append(errs, errors.New("jwks.rotation.grace_period must be at least auth.access_token_ttl"))
	}
	if rotation.MaxPublishableKey < 2 {
		errs = append(errs, errors.New("jwks.rotation.max_publishable_keys must be at least 2"))
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(strings.TrimSpace(rotation.CheckCron)); err != nil {
		errs = append(errs, errors.New("jwks.rotation.check_cron must be a valid cron expression"))
	}
	if o.GenericServerRunOptions != nil {
		if profile, err := o.GenericServerRunOptions.RuntimeProfile(); err == nil && profile.IsProductionLike() {
			keysDir := strings.TrimSpace(o.JWKS.KeysDir)
			if keysDir == "" || !filepath.IsAbs(keysDir) {
				errs = append(errs, errors.New("jwks.keys_dir must be a non-empty absolute path in release mode"))
			}
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
		{key: "IAM_APISERVER_SUGGEST_DATA_DIR", message: "IAM_APISERVER_SUGGEST_DATA_DIR has been removed; suggest indexes are memory-only"},
		{key: "IAM_APISERVER_SUGGEST_SNAPSHOT", message: "IAM_APISERVER_SUGGEST_SNAPSHOT has been removed; suggest indexes are not persisted to files"},
	} {
		if _, set := os.LookupEnv(removed.key); set {
			errs = append(errs, errors.New(removed.message))
		}
	}
	return errs
}
