package options

import (
	"errors"

	"github.com/FangcunMount/iam/v2/internal/pkg/server"
	"github.com/spf13/pflag"
)

// ServerRunOptions 在运行的通用服务器选项
type ServerRunOptions struct {
	Mode                 string   `json:"mode" mapstructure:"mode"`
	Healthz              bool     `json:"healthz" mapstructure:"healthz"`
	Middlewares          []string `json:"middlewares" mapstructure:"middlewares"`
	AllowDegradedStartup bool     `json:"allowDegradedStartup" mapstructure:"allow-degraded-startup"`
	RemovedRunMode       *string  `json:"-" mapstructure:"run-mode"`
	RemovedName          *string  `json:"-" mapstructure:"name"`
	RemovedReadTimeout   *int     `json:"-" mapstructure:"read-timeout"`
	RemovedWriteTimeout  *int     `json:"-" mapstructure:"write-timeout"`
}

// NewServerRunOptions 简单工厂方法，创建在运行的服务器选项
func NewServerRunOptions() *ServerRunOptions {
	defaults := server.NewConfig()

	return &ServerRunOptions{
		Mode:                 defaults.Mode,
		Healthz:              defaults.Healthz,
		Middlewares:          defaults.Middlewares,
		AllowDegradedStartup: false,
	}
}

// Complete normalizes the configured mode after validating it.
func (s *ServerRunOptions) Complete() error {
	profile, err := s.RuntimeProfile()
	if err != nil {
		return err
	}
	s.Mode = string(profile.ServerMode)
	return nil
}

// RuntimeProfile resolves the canonical runtime profile for these options.
func (s *ServerRunOptions) RuntimeProfile() (server.RuntimeProfile, error) {
	if s == nil {
		return server.ResolveRuntimeProfile(string(server.RuntimeModeRelease))
	}
	return server.ResolveRuntimeProfile(s.Mode)
}

// ApplyTo 将运行选项应用到方法接收者并返回自身
func (s *ServerRunOptions) ApplyTo(c *server.Config) error {
	profile, err := s.RuntimeProfile()
	if err != nil {
		return err
	}
	c.Mode = string(profile.ServerMode)
	c.Healthz = s.Healthz
	c.Middlewares = s.Middlewares

	return nil
}

// Validate 检查 ServerRunOptions 的验证
func (s *ServerRunOptions) Validate() []error {
	var errs []error
	if s == nil {
		return errs
	}
	for _, removed := range []struct {
		set     bool
		message string
	}{
		{
			set:     s.RemovedRunMode != nil,
			message: "server.run-mode has been removed; use server.mode",
		},
		{
			set:     s.RemovedName != nil,
			message: "server.name has been removed and has no replacement",
		},
		{
			set:     s.RemovedReadTimeout != nil,
			message: "server.read-timeout has been removed; HTTP read timeout is not configurable",
		},
		{
			set:     s.RemovedWriteTimeout != nil,
			message: "server.write-timeout has been removed; HTTP write timeout is not configurable",
		},
	} {
		if removed.set {
			errs = append(errs, errors.New(removed.message))
		}
	}
	if _, err := s.RuntimeProfile(); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// AddFlags 为特定的 APIServer 添加标志到指定的 FlagSet
func (s *ServerRunOptions) AddFlags(fs *pflag.FlagSet) {
	// Note: the weird ""+ in below lines seems to be the only way to get gofmt to
	// arrange these text blocks sensibly. Grrr.
	fs.StringVar(&s.Mode, "server.mode", s.Mode, ""+
		"Start the server in a specified server mode. Supported server mode: debug, test, release.")

	fs.BoolVar(&s.Healthz, "server.healthz", s.Healthz, ""+
		"Add self readiness check and install /healthz router.")

	fs.StringSliceVar(&s.Middlewares, "server.middlewares", s.Middlewares, ""+
		"List of allowed middlewares for server, comma separated. If this list is empty default middlewares will be used.")

	fs.BoolVar(&s.AllowDegradedStartup, "server.allow-degraded-startup", s.AllowDegradedStartup, ""+
		"Allow partial startup when critical modules are unavailable. This is rejected in production/release mode.")
}
