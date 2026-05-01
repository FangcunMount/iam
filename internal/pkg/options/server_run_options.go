package options

import (
	"github.com/FangcunMount/iam/v2/internal/pkg/server"
	"github.com/spf13/pflag"
)

// ServerRunOptions 在运行的通用服务器选项
type ServerRunOptions struct {
	Mode                 string   `json:"mode" mapstructure:"mode"`
	Healthz              bool     `json:"healthz" mapstructure:"healthz"`
	Middlewares          []string `json:"middlewares" mapstructure:"middlewares"`
	AllowDegradedStartup bool     `json:"allowDegradedStartup" mapstructure:"allow-degraded-startup"`
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

// ApplyTo 将运行选项应用到方法接收者并返回自身
func (s *ServerRunOptions) ApplyTo(c *server.Config) error {
	c.Mode = s.Mode
	c.Healthz = s.Healthz
	c.Middlewares = s.Middlewares

	return nil
}

// Validate 检查 ServerRunOptions 的验证
func (s *ServerRunOptions) Validate() []error {
	errors := []error{}

	return errors
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
